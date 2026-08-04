package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskUsername(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "***"},
		{name: "email long local", in: "zhang123@x.com", want: "z***3@x.com"},
		{name: "email short local", in: "ab@example.com", want: "***@example.com"},
		{name: "email single char local", in: "a@example.com", want: "***@example.com"},
		{name: "phone full", in: "13812345678", want: "138****5678"},
		{name: "phone 4 digits", in: "1234", want: "1***4"},
		{name: "phone 3 digits", in: "123", want: "***"},
		{name: "plain handle", in: "alice", want: "alice"},
		{name: "plain handle with underscore", in: "user_name", want: "user_name"},
		{name: "unicode display name", in: "张三", want: "张三"},
		{name: "whitespace around phone", in: " 13812345678 ", want: "138****5678"},
		{name: "non-digit phone-like", in: "+8613812345678", want: "+8613812345678"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MaskUsername(tt.in))
		})
	}
}
