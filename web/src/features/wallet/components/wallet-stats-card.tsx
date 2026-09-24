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
import { Activity, WalletCards, Zap, Gem } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import type { UserWalletData } from '../types'

interface WalletStatsCardProps {
  user: UserWalletData | null
  loading?: boolean
}

export function WalletStatsCard(props: WalletStatsCardProps) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='grid grid-cols-2 md:grid-cols-4 divide-x rounded-lg border'>
        {['power', 'anlas', 'requests', 'balance'].map((key) => (
          <div key={key} className='min-w-0 px-2.5 py-2.5 sm:px-5 sm:py-4'>
            <Skeleton className='h-3.5 w-full' />
            <Skeleton className='mt-2 h-6 w-full sm:h-7' />
            <Skeleton className='mt-1.5 hidden h-3.5 w-24 md:block' />
          </div>
        ))}
      </div>
    )
  }

  const power = props.user?.power ?? 0
  const anlas = props.user?.anlas ?? 0

  const stats: {
    label: string
    value: string
    description: string
    icon: typeof WalletCards
    tone: IconBadgeTone
    isNegative?: boolean
  }[] = [
    {
      label: '⚡ 每日电量 (Power)',
      value: power.toLocaleString(),
      description: props.user?.auto_power_enabled
        ? `每日自动刷新至 ${props.user?.auto_power_amount}`
        : 'V5 电池电量 (标准单张)',
      icon: Zap,
      tone: 'warning',
      isNegative: power < 0,
    },
    {
      label: '💎 Opus 点数 (Anlas)',
      value: anlas.toLocaleString(),
      description: props.user?.auto_anlas_enabled
        ? `每月自动分配 +${(props.user?.auto_anlas_amount ?? 10000).toLocaleString()}`
        : '高步数/超大图/多图消耗',
      icon: Gem,
      tone: 'info',
      isNegative: anlas < 0,
    },
    {
      label: t('API Requests'),
      value: (props.user?.request_count ?? 0).toLocaleString(),
      description: t('Total requests made'),
      icon: Activity,
      tone: 'chart-4',
    },
    {
      label: t('Current Balance'),
      value: formatQuota(props.user?.quota ?? 0),
      description: t('Remaining quota'),
      icon: WalletCards,
      tone: 'success',
    },
  ]

  return (
    <div className='grid grid-cols-2 md:grid-cols-4 divide-x rounded-lg border'>
      {stats.map((item) => (
        <div key={item.label} className='min-w-0 px-2.5 py-2.5 sm:px-5 sm:py-4'>
          <div className='flex items-center gap-1.5 sm:gap-2.5'>
            <IconBadge tone={item.tone} size='stat'>
              <item.icon />
            </IconBadge>
            <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
              {item.label}
            </div>
          </div>

          <div
            className={`mt-1.5 font-mono text-sm font-bold tracking-tight break-all tabular-nums sm:mt-2.5 sm:text-2xl ${
              item.isNegative ? 'text-destructive' : 'text-foreground'
            }`}
          >
            {item.value}
            {item.isNegative && (
              <span className='ml-1 text-xs text-destructive font-normal'>
                (欠费)
              </span>
            )}
          </div>
          <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
            {item.description}
          </div>
        </div>
      ))}
    </div>
  )
}
