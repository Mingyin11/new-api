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
import { Zap, Gem, DollarSign } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { adjustUserQuota } from '../api'
import type { QuotaAdjustMode } from '../types'

export type QuotaTargetType = 'power' | 'anlas' | 'quota'

interface UserQuotaDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  currentQuota: number
  currentPower?: number
  currentAnlas?: number
  initialTargetType?: QuotaTargetType
  onSuccess: () => void
}

export function UserQuotaDialog(props: UserQuotaDialogProps) {
  const { t } = useTranslation()
  const [targetType, setTargetType] = useState<QuotaTargetType>(
    props.initialTargetType || 'power'
  )
  const [mode, setMode] = useState<QuotaAdjustMode>('add')
  const [amount, setAmount] = useState('')
  const [loading, setLoading] = useState(false)
  const refreshAuth = useAuthStore((s) => s.refreshAuth)
  const currentAuthUser = useAuthStore((s) => s.auth.user)

  useEffect(() => {
    if (props.open && props.initialTargetType) {
      setTargetType(props.initialTargetType)
    }
  }, [props.open, props.initialTargetType])

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'

  const amountValue = Number.parseFloat(amount) || 0
  const quotaValue = parseQuotaFromDollars(Math.abs(amountValue))
  const intAmountValue = Math.floor(Math.abs(amountValue))

  const getPreviewText = () => {
    if (targetType === 'power') {
      const current = props.currentPower ?? 0
      const val = intAmountValue
      switch (mode) {
        case 'add':
          return `⚡ 当前电量: ${current} + ${val} = ${current + val}`
        case 'subtract':
          return `⚡ 当前电量: ${current} - ${val} = ${current - val}`
        case 'override':
          return `⚡ 当前电量: ${current} → ${Number.parseInt(amount, 10) || 0}`
        default:
          return ''
      }
    }
    if (targetType === 'anlas') {
      const current = props.currentAnlas ?? 0
      const val = intAmountValue
      switch (mode) {
        case 'add':
          return `💎 当前 Anlas: ${current.toLocaleString()} + ${val.toLocaleString()} = ${(current + val).toLocaleString()}`
        case 'subtract':
          return `💎 当前 Anlas: ${current.toLocaleString()} - ${val.toLocaleString()} = ${(current - val).toLocaleString()}`
        case 'override':
          return `💎 当前 Anlas: ${current.toLocaleString()} → ${(Number.parseInt(amount, 10) || 0).toLocaleString()}`
        default:
          return ''
      }
    }
    const current = props.currentQuota
    const val = quotaValue
    switch (mode) {
      case 'add':
        return `${t('Current quota')}: ${formatQuota(current)}  +${formatQuota(val)} = ${formatQuota(current + val)}`
      case 'subtract':
        return `${t('Current quota')}: ${formatQuota(current)}  -${formatQuota(val)} = ${formatQuota(current - val)}`
      case 'override': {
        const overrideQuota = parseQuotaFromDollars(amountValue)
        return `${t('Current quota')}: ${formatQuota(current)} → ${formatQuota(overrideQuota)}`
      }
      default:
        return ''
    }
  }

  const handleConfirm = async () => {
    if (!amount && mode !== 'override') return

    setLoading(true)
    try {
      let finalValue: number
      if (targetType === 'power' || targetType === 'anlas') {
        const intVal = Number.parseInt(amount, 10) || 0
        finalValue = mode === 'override' ? intVal : Math.abs(intVal)
      } else {
        finalValue = mode === 'override' ? parseQuotaFromDollars(amountValue) : quotaValue
      }

      const result = await adjustUserQuota({
        id: props.userId,
        action: 'add_quota',
        mode,
        value: finalValue,
        target_type: targetType,
      })

      if (result.success) {
        const typeLabel =
          targetType === 'power' ? '电量 (Power)' : targetType === 'anlas' ? 'Opus 点数 (Anlas)' : t('Quota')
        toast.success(`${typeLabel} 调整成功`)
        setAmount('')
        setMode('add')
        props.onOpenChange(false)
        props.onSuccess()
        // If modifying current logged-in user, refresh auth so wallet updates instantly
        if (currentAuthUser?.id === props.userId) {
          void refreshAuth()
        }
      } else {
        handleServerError(result, t('Failed to adjust quota'))
      }
    } catch (e: unknown) {
      handleServerError(e, t('Failed to adjust quota'))
    } finally {
      setLoading(false)
    }
  }

  const handleCancel = () => {
    setAmount('')
    setMode('add')
    props.onOpenChange(false)
  }

  const placeholder =
    targetType === 'power'
      ? '请输入电量数值 (如: 50)'
      : targetType === 'anlas'
        ? '请输入 Anlas 点数 (如: 10000)'
        : tokensOnly
          ? t('Enter amount in tokens')
          : t('Enter amount in {{currency}}', { currency: currencyLabel })

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title='用户配额与点数管理'
      description='支持调整 NovelAI 每日电量 (Power)、Opus 点数 (Anlas) 及通用额度'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={handleCancel}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={loading}>
            {loading ? t('Processing...') : t('Confirm')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        {/* Target Type Selector */}
        <div className='space-y-2'>
          <Label className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
            选择配额类型
          </Label>
          <div className='grid grid-cols-3 gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              className={cn(
                'flex items-center gap-1.5',
                targetType === 'power' &&
                  'bg-amber-500/10 text-amber-600 border-amber-500/40 dark:text-amber-400 font-medium'
              )}
              onClick={() => {
                setTargetType('power')
                setAmount('')
              }}
            >
              <Zap className='h-3.5 w-3.5 text-amber-500' />
              <span>⚡ 电量 (Power)</span>
            </Button>

            <Button
              type='button'
              variant='outline'
              size='sm'
              className={cn(
                'flex items-center gap-1.5',
                targetType === 'anlas' &&
                  'bg-cyan-500/10 text-cyan-600 border-cyan-500/40 dark:text-cyan-400 font-medium'
              )}
              onClick={() => {
                setTargetType('anlas')
                setAmount('')
              }}
            >
              <Gem className='h-3.5 w-3.5 text-cyan-500' />
              <span>💎 Anlas 点数</span>
            </Button>

            <Button
              type='button'
              variant='outline'
              size='sm'
              className={cn(
                'flex items-center gap-1.5',
                targetType === 'quota' &&
                  'bg-emerald-500/10 text-emerald-600 border-emerald-500/40 dark:text-emerald-400 font-medium'
              )}
              onClick={() => {
                setTargetType('quota')
                setAmount('')
              }}
            >
              <DollarSign className='h-3.5 w-3.5 text-emerald-500' />
              <span>通用额度 (USD)</span>
            </Button>
          </div>
        </div>

        <div className='rounded-md border bg-muted/40 p-2.5 text-xs text-muted-foreground font-mono'>
          {getPreviewText()}
        </div>

        <div className='space-y-2'>
          <Label>{t('Mode')}</Label>
          <div className='flex gap-1'>
            {(['add', 'subtract', 'override'] as const).map((m) => (
              <Button
                key={m}
                type='button'
                variant='outline'
                size='sm'
                className={cn(
                  mode === m &&
                    'bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                )}
                onClick={() => {
                  setMode(m)
                  setAmount('')
                }}
              >
                {m === 'add' && t('Add')}
                {!(m === 'add') && m === 'subtract' && t('Subtract')}
                {!(m === 'add') && !(m === 'subtract') && t('Override')}
              </Button>
            ))}
          </div>
        </div>

        <div className='space-y-2'>
          <Label>
            {targetType === 'power'
              ? '变动电量数 (整张数)'
              : targetType === 'anlas'
                ? '变动 Anlas 点数'
                : `${t('Amount')} (${currencyLabel})`}
          </Label>
          <Input
            type='number'
            step={targetType === 'power' || targetType === 'anlas' ? 1 : tokensOnly ? 1 : 0.000001}
            min={mode === 'override' ? undefined : 0}
            placeholder={placeholder}
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleConfirm()
            }}
          />
        </div>
      </div>
    </Dialog>
  )
}
