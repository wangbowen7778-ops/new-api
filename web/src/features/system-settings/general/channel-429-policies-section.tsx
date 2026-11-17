/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'
import { SettingsForm } from '../components/settings-form-layout'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const channel429PoliciesSchema = z.object({
  Channel429Policies: z.string(),
})

type Channel429PoliciesFormValues = z.infer<typeof channel429PoliciesSchema>

type Channel429PoliciesSectionProps = {
  defaultValues: Channel429PoliciesFormValues
}

const POLICIES_EXAMPLE = `[
  {
    "match": ["token rate limit", "rate limit exceeded"],
    "action": "retry",
    "cooldown_base": 10,
    "cooldown_max": 120
  },
  {
    "match": ["exceeded your current quota", "credit balance"],
    "action": "retry",
    "cooldown_base": 60,
    "cooldown_max": 300
  }
]`

export function Channel429PoliciesSection({
  defaultValues,
}: Channel429PoliciesSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm({
    resolver: zodResolver(channel429PoliciesSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (data: Channel429PoliciesFormValues) => {
    const value = data.Channel429Policies?.trim() ?? ''
    // Reject malformed JSON before it reaches the backend validator, so the
    // operator sees the problem next to the editor instead of a silent 4xx.
    if (value !== '') {
      try {
        const parsed = JSON.parse(value)
        if (!Array.isArray(parsed)) {
          form.setError('Channel429Policies', {
            message: t('Channel 429 policies must be a JSON array'),
          })
          return
        }
      } catch {
        form.setError('Channel429Policies', {
          message: t('Channel 429 policies must be valid JSON'),
        })
        return
      }
    }
    await updateOption.mutateAsync({
      key: 'Channel429Policies',
      value,
    })
  }

  return (
    <SettingsSection title={t('Channel 429 Policies')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
              control={form.control}
              name='Channel429Policies'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Channel 429 Policies')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={POLICIES_EXAMPLE}
                      rows={12}
                      className='font-mono text-xs'
                      spellCheck={false}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Map upstream 429 error keywords to a strategy. action: retry (cooldown then switch channel), retry_no_cooldown (switch channel immediately, no cooldown), cooldown_only (fail fast) or wait_retry (sleep on the same channel for the upstream retry-after, capped by wait_max_seconds). Unmatched 429s keep the built-in 3s-60s retry cooldown.',
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
