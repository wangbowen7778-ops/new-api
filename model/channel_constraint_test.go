package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterCandidateIDs(t *testing.T) {
	alphaSetting := `{"task_plugin_key":"alpha"}`
	betaSetting := `{"task_plugin_key":"beta"}`
	alpha := &Channel{Id: 900001, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Setting: &alphaSetting}
	beta := &Channel{Id: 900002, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Setting: &betaSetting}
	ordinary := &Channel{Id: 900003, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled}
	kling := &Channel{Id: 900004, Type: constant.ChannelTypeKling, Status: common.ChannelStatusEnabled}
	jimeng := &Channel{Id: 900005, Type: constant.ChannelTypeJimeng, Status: common.ChannelStatusEnabled}
	matchingCustom := &Channel{Id: 900010, Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled}
	matchingCustom.SetOtherSettings(kitdto.ChannelOtherSettings{
		AdvancedCustom: &kitdto.AdvancedCustomConfig{
			Routes: []kitdto.AdvancedCustomRoute{{
				IncomingPath: "/v1/chat/completions",
				Models:       []string{"gpt-4"},
			}},
		},
	})
	otherCustom := &Channel{Id: 900011, Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled}
	otherCustom.SetOtherSettings(kitdto.ChannelOtherSettings{
		AdvancedCustom: &kitdto.AdvancedCustomConfig{
			Routes: []kitdto.AdvancedCustomRoute{{
				IncomingPath: "/v1/responses",
				Models:       []string{"gpt-4"},
			}},
		},
	})
	videoSetting := `{"video_parsing_enabled":true}`
	video := &Channel{Id: 900020, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Setting: &videoSetting}

	pathFilter := dto.ChannelFilter{Kind: dto.FilterRequestPath, RequestPath: "/v1/chat/completions"}
	emptyPathFilter := dto.ChannelFilter{Kind: dto.FilterRequestPath, RequestPath: ""}

	tests := []struct {
		name      string
		ids       []int
		modelName string
		filters   []dto.ChannelFilter
		wantKept  []int
		wantEmpty dto.ChannelFilterKind
	}{
		{
			name:      "identity keeps matching type-59 key",
			ids:       []int{900001, 900002},
			modelName: "shared",
			filters:   identityFilters("alpha", nil),
			wantKept:  []int{900001},
		},
		{
			name:      "identity empty key drops all type-59",
			ids:       []int{900001, 900002},
			modelName: "shared",
			filters:   identityFilters("", nil),
			wantKept:  []int{},
			wantEmpty: dto.FilterTaskPluginIdentity,
		},
		{
			name:      "identity empty key keeps ordinary channel",
			ids:       []int{900003},
			modelName: "ordinary",
			filters:   identityFilters("", nil),
			wantKept:  []int{900003},
		},
		{
			name:      "identity keeps matching legacy type",
			ids:       []int{900004, 900005},
			modelName: "legacy",
			filters:   identityFilters("legacy-alpha", []int{constant.ChannelTypeKling}),
			wantKept:  []int{900004},
		},
		{
			name:      "identity keeps all listed legacy types",
			ids:       []int{900004, 900005},
			modelName: "legacy",
			filters:   identityFilters("legacy-alpha", []int{constant.ChannelTypeKling, constant.ChannelTypeJimeng}),
			wantKept:  []int{900004, 900005},
		},
		{
			name:      "identity keyed with no types drops legacy",
			ids:       []int{900004, 900005},
			modelName: "legacy",
			filters:   identityFilters("legacy-alpha", nil),
			wantKept:  []int{},
			wantEmpty: dto.FilterTaskPluginIdentity,
		},
		{
			name:      "identity drops missing cache entry",
			ids:       []int{900004, 999999},
			modelName: "legacy",
			filters:   identityFilters("legacy-alpha", []int{constant.ChannelTypeKling}),
			wantKept:  []int{900004},
		},
		{
			name:      "empty request path is a passthrough including missing ids",
			ids:       []int{900003, 900010, 999999},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{emptyPathFilter},
			wantKept:  []int{900003, 900010, 999999},
		},
		{
			name:      "request path keeps missing cache entry for consistency",
			ids:       []int{900003, 999999},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{pathFilter},
			wantKept:  []int{900003, 999999},
		},
		{
			name:      "request path keeps matching type-58 and ordinary",
			ids:       []int{900003, 900010, 900011},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{pathFilter},
			wantKept:  []int{900003, 900010},
		},
		{
			name:      "request path empties when only unmatched type-58 remains",
			ids:       []int{900011},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{pathFilter},
			wantKept:  []int{},
			wantEmpty: dto.FilterRequestPath,
		},
		{
			name:      "intersection attributes empty set to identity after path keeps candidates",
			ids:       []int{900001, 900010},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{pathFilter, identityFilters("missing", nil)[0]},
			wantKept:  []int{},
			wantEmpty: dto.FilterTaskPluginIdentity,
		},
		{
			name:      "intersection attributes empty set to path when path runs first",
			ids:       []int{900011},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{identityFilters("", nil)[0], pathFilter},
			wantKept:  []int{},
			wantEmpty: dto.FilterRequestPath,
		},
		{
			name:      "video filter keeps only video-marked channel",
			ids:       []int{900003, 900020},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{{Kind: dto.FilterRequestContainsVideo}},
			wantKept:  []int{900020},
		},
		{
			name:      "video filter drops all unmarked channels",
			ids:       []int{900003},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{{Kind: dto.FilterRequestContainsVideo}},
			wantKept:  []int{},
			wantEmpty: dto.FilterRequestContainsVideo,
		},
		{
			name:      "video filter drops missing cache entry",
			ids:       []int{900020, 999999},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{{Kind: dto.FilterRequestContainsVideo}},
			wantKept:  []int{900020},
		},
		{
			name:      "no video filter keeps unmarked channels",
			ids:       []int{900003, 900020},
			modelName: "gpt-4",
			filters:   nil,
			wantKept:  []int{900003, 900020},
		},
		{
			name:      "exclude filter drops the tried channel",
			ids:       []int{900003, 900004},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{{Kind: dto.FilterExcludeChannels, ChannelIDs: []int{900003}}},
			wantKept:  []int{900004},
		},
		{
			name:      "exclude filter reports the kind when every candidate was tried",
			ids:       []int{900003},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{{Kind: dto.FilterExcludeChannels, ChannelIDs: []int{900003}}},
			wantKept:  []int{},
			wantEmpty: dto.FilterExcludeChannels,
		},
		{
			name:      "exclude filter keeps unrelated candidates",
			ids:       []int{900004},
			modelName: "gpt-4",
			filters:   []dto.ChannelFilter{{Kind: dto.FilterExcludeChannels, ChannelIDs: []int{900003}}},
			wantKept:  []int{900004},
		},
	}

	channelSyncLock.Lock()
	previous := channelsIDM
	channelsIDM = map[int]*Channel{
		900001: alpha,
		900002: beta,
		900003: ordinary,
		900004: kling,
		900005: jimeng,
		900010: matchingCustom,
		900011: otherCustom,
		900020: video,
	}
	t.Cleanup(func() {
		channelsIDM = previous
		channelSyncLock.Unlock()
	})

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			kept, emptiedBy := filterCandidateIDs(testCase.ids, testCase.modelName, testCase.filters)
			if testCase.wantKept == nil {
				assert.Nil(t, kept)
			} else {
				assert.Equal(t, testCase.wantKept, kept)
			}
			assert.Equal(t, testCase.wantEmpty, emptiedBy)
		})
	}
}

func TestBuildPriorityTiers(t *testing.T) {
	nonVideo := func(priority int64) channelTier { return channelTier{priority: priority} }
	video := func(priority int64) channelTier { return channelTier{priority: priority, video: true} }

	tierPriorities := func(tiers []channelTier) []int64 {
		priorities := make([]int64, 0, len(tiers))
		for _, tier := range tiers {
			priorities = append(priorities, tier.priority)
		}
		return priorities
	}

	tests := []struct {
		name           string
		candidates     []channelTier
		preferNonVideo bool
		want           []int64
	}{
		{
			name:       "without the preference one tier per distinct priority",
			candidates: []channelTier{nonVideo(10), video(100), nonVideo(100)},
			want:       []int64{100, 10},
		},
		{
			name:           "preference orders every non-video tier ahead of every video tier",
			candidates:     []channelTier{nonVideo(10), video(100)},
			preferNonVideo: true,
			want:           []int64{10, 100},
		},
		{
			name:           "a priority holding both lanes splits into a non-video tier then a video tier",
			candidates:     []channelTier{video(100), nonVideo(100)},
			preferNonVideo: true,
			want:           []int64{100, 100},
		},
		{
			name:           "each lane still descends by priority",
			candidates:     []channelTier{nonVideo(5), nonVideo(50), video(10), video(60)},
			preferNonVideo: true,
			want:           []int64{50, 5, 60, 10},
		},
		{
			name:           "video-only requests keep the plain priority order",
			candidates:     []channelTier{nonVideo(10), video(100)},
			preferNonVideo: false,
			want:           []int64{100, 10},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, tierPriorities(buildPriorityTiers(testCase.candidates, testCase.preferNonVideo)))
		})
	}

	splitsLanes := buildPriorityTiers([]channelTier{video(100), nonVideo(100)}, true)
	require.Len(t, splitsLanes, 2)
	assert.True(t, splitsLanes[0].matchesChannel(false), "the non-video tier admits only non-video channels")
	assert.False(t, splitsLanes[0].matchesChannel(true))
	assert.True(t, splitsLanes[1].matchesChannel(true), "the video tier admits only video channels")
	assert.False(t, splitsLanes[1].matchesChannel(false))

	plain := buildPriorityTiers([]channelTier{video(100), nonVideo(100)}, false)
	require.Len(t, plain, 1)
	assert.True(t, plain[0].matchesChannel(true), "without the preference a tier is lane-agnostic")
	assert.True(t, plain[0].matchesChannel(false))
}

func TestGetRandomSatisfiedChannelPrefersNonVideo(t *testing.T) {
	truncateTables(t)
	originalMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCache; InitChannelCache() })

	videoSetting := `{"video_parsing_enabled":true}`
	insert := func(id int, priority int64, setting *string) {
		weight := uint(1)
		require.NoError(t, (&Channel{
			Id: id, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled,
			Name: fmt.Sprintf("channel-%d", id), Models: "vm", Group: "default",
			Priority: &priority, Weight: &weight, Setting: setting,
		}).Insert())
	}
	insert(930001, 100, &videoSetting)
	insert(930002, 10, nil)
	insert(930003, 5, nil)
	insert(930004, 50, &videoSetting)
	InitChannelCache()

	preference := []dto.ChannelFilter{{Kind: dto.FilterNonVideoPreferred}}

	selected, err := GetRandomSatisfiedChannel("default", "vm", 0, preference)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 930002, selected.Id, "the top non-video tier wins even at a lower priority")

	selected, err = GetRandomSatisfiedChannel("default", "vm", 1, preference)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 930003, selected.Id, "every non-video tier is exhausted before any video tier")

	selected, err = GetRandomSatisfiedChannel("default", "vm", 2, preference)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 930001, selected.Id, "the video lane still descends by priority")

	selected, err = GetRandomSatisfiedChannel("default", "vm", 3, preference)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 930004, selected.Id)

	selected, err = GetRandomSatisfiedChannel("default", "vm", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 930001, selected.Id, "an unclassified request keeps the highest-priority tier")

	selected, err = GetRandomSatisfiedChannel("default", "vm", 1, nil)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 930004, selected.Id, "without the preference priority alone orders the tiers")

	selected, err = GetRandomSatisfiedChannel("default", "vm", 0, []dto.ChannelFilter{{Kind: dto.FilterRequestContainsVideo}})
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 930001, selected.Id, "a video request only sees video-parsing channels")

	assert.Equal(t, 4, MaxConcurrencyRetryProbes("default", "vm", preference), "the two lanes contribute a tier each")
	assert.Equal(t, 4, MaxConcurrencyRetryProbes("default", "vm", nil))
}

func TestChannelSatisfiesFilters(t *testing.T) {
	alphaSetting := `{"task_plugin_key":"alpha"}`
	alpha := &Channel{Id: 1, Type: constant.ChannelTypeTaskPlugin, Setting: &alphaSetting}
	ordinary := &Channel{Id: 2, Type: constant.ChannelTypeOpenAI}
	custom := &Channel{Id: 3, Type: constant.ChannelTypeAdvancedCustom}
	custom.SetOtherSettings(kitdto.ChannelOtherSettings{
		AdvancedCustom: &kitdto.AdvancedCustomConfig{
			Routes: []kitdto.AdvancedCustomRoute{{
				IncomingPath: "/v1/chat/completions",
				Models:       []string{"gpt-4"},
			}},
		},
	})

	ok, kind := ChannelSatisfiesFilters(nil, "gpt-4", nil)
	assert.False(t, ok)
	assert.Equal(t, dto.ChannelFilterKind(""), kind)

	ok, kind = ChannelSatisfiesFilters(alpha, "shared", identityFilters("alpha", nil))
	require.True(t, ok)
	assert.Equal(t, dto.ChannelFilterKind(""), kind)

	ok, kind = ChannelSatisfiesFilters(alpha, "shared", identityFilters("beta", nil))
	assert.False(t, ok)
	assert.Equal(t, dto.FilterTaskPluginIdentity, kind)

	ok, kind = ChannelSatisfiesFilters(ordinary, "gpt-4", []dto.ChannelFilter{{
		Kind:        dto.FilterRequestPath,
		RequestPath: "/v1/chat/completions",
	}})
	require.True(t, ok)
	assert.Equal(t, dto.ChannelFilterKind(""), kind)

	ok, kind = ChannelSatisfiesFilters(custom, "gpt-4", []dto.ChannelFilter{{
		Kind:        dto.FilterRequestPath,
		RequestPath: "/v1/responses",
	}})
	assert.False(t, ok)
	assert.Equal(t, dto.FilterRequestPath, kind)
}
