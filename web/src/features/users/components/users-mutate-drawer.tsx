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
import { useQuery } from '@tanstack/react-query'
import { Pencil, Zap, Gem } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Combobox } from '@/components/ui/combobox'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  EMPTY_PERMISSION_CATALOG,
  hasPermission,
  normalizeAdminPermissions,
} from '@/lib/admin-permissions'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import { accountPasswordSchema } from '@/lib/password-policy'
import { ROLE } from '@/lib/roles'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import {
  createUser,
  updateUser,
  getUser,
  getGroups,
  getPermissionCatalog,
} from '../api'
import { BINDING_FIELDS, ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  userFormSchema,
  type UserFormValues,
  USER_FORM_DEFAULT_VALUES,
  transformFormDataToPayload,
  transformUserToFormDefaults,
} from '../lib'
import type { User } from '../types'
import { UserQuotaDialog, type QuotaTargetType } from './user-quota-dialog'
import { useUsers } from './users-provider'

type UsersMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: User
}

export function UsersMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: UsersMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { triggerRefresh } = useUsers()
  const currentUser = useAuthStore((s) => s.auth.user)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [quotaDialogOpen, setQuotaDialogOpen] = useState(false)
  const [quotaDialogTargetType, setQuotaDialogTargetType] =
    useState<QuotaTargetType>('power')

  // Fetch groups
  const { data: groupsData } = useQuery({
    queryKey: ['groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
    staleTime: 5 * 60 * 1000,
  })

  const groups = groupsData?.data || []

  // Permission catalog is owned by the backend; fetched once and reused.
  const { data: permissionCatalog = EMPTY_PERMISSION_CATALOG } = useQuery({
    queryKey: ['admin-permission-catalog'],
    queryFn: async () => requireServerSuccess(await getPermissionCatalog()),
    staleTime: 5 * 60 * 1000,
  })

  const form = useForm<UserFormValues>({
    resolver: zodResolver(userFormSchema),
    defaultValues: USER_FORM_DEFAULT_VALUES,
  })

  // Load existing data when updating
  useEffect(() => {
    if (open && isUpdate && currentRow) {
      // For update, fetch fresh data
      getUser(currentRow.id)
        .then((result) => {
          if (result.success && result.data) {
            form.reset(transformUserToFormDefaults(result.data))
          } else {
            handleServerError(result, t('Failed to load'))
          }
        })
        .catch((error) => handleServerError(error, t('Failed to load')))
    } else if (open && !isUpdate) {
      // For create, reset to defaults
      form.reset(USER_FORM_DEFAULT_VALUES)
    }
  }, [open, isUpdate, currentRow, form, t])

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'

  const currentQuotaRaw = form.watch('quota_dollars') || 0
  const selectedRole = form.watch('role')
  const canEditAdminPermissions = currentUser?.role === ROLE.SUPER_ADMIN
  const targetIsAdmin = (selectedRole ?? currentRow?.role ?? 0) >= ROLE.ADMIN

  const onSubmit = async (data: UserFormValues) => {
    if (!isUpdate || data.password) {
      if (!accountPasswordSchema.safeParse(data.password ?? '').success) {
        form.setError('password', {
          type: 'manual',
          message: t('Password must contain between 8 and 128 characters.'),
        })
        return
      }
    }

    setIsSubmitting(true)
    try {
      const payload = transformFormDataToPayload(
        data,
        currentRow?.id,
        permissionCatalog
      )
      const result = isUpdate
        ? await updateUser(payload as typeof payload & { id: number })
        : await createUser(payload)

      if (result.success) {
        toast.success(
          isUpdate
            ? t(SUCCESS_MESSAGES.USER_UPDATED)
            : t(SUCCESS_MESSAGES.USER_CREATED)
        )
        onOpenChange(false)
        triggerRefresh()
      } else {
        handleServerError(result, t(ERROR_MESSAGES.CREATE_FAILED))
      }
    } catch (error) {
      handleServerError(error, t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsSubmitting(false)
    }
  }

  const refreshUserData = async () => {
    if (!currentRow) return
    try {
      const result = requireServerSuccess(await getUser(currentRow.id))
      if (result.success && result.data) {
        form.reset(transformUserToFormDefaults(result.data))
      }
      triggerRefresh()
    } catch (error) {
      handleServerError(error, t('Failed to load'))
    }
  }

  return (
    <>
      <Sheet
        open={open}
        onOpenChange={(v) => {
          onOpenChange(v)
          if (!v) {
            form.reset()
          }
        }}
      >
        <SheetContent
          className={sideDrawerContentClassName('sm:max-w-[600px]')}
        >
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>
              {isUpdate ? t('Update') : t('Create')} {t('User')}
            </SheetTitle>
            <SheetDescription>
              {isUpdate
                ? t('Update the user by providing necessary info.')
                : t('Add a new user by providing necessary info.')}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id='user-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className={sideDrawerFormClassName()}
            >
              {/* Basic Information */}
              <SideDrawerSection>
                <h3 className='text-sm font-medium'>
                  {t('Basic Information')}
                </h3>

                <FormField
                  control={form.control}
                  name='username'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Username')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Enter username')}
                          disabled={isUpdate}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {!isUpdate && (
                  <FormField
                    control={form.control}
                    name='role'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Role')}</FormLabel>
                        <Select
                          items={[
                            { value: '1', label: t('Common User') },
                            { value: '10', label: t('Admin') },
                          ]}
                          onValueChange={(value) =>
                            value !== null &&
                            field.onChange(Number.parseInt(value))
                          }
                          value={String(field.value)}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t('Select a role')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              <SelectItem value='1'>
                                {t('Common User')}
                              </SelectItem>
                              <SelectItem value='10'>{t('Admin')}</SelectItem>
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormDescription>
                          {t("Set the user's role (cannot be Root)")}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                <FormField
                  control={form.control}
                  name='display_name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Display Name')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Enter display name')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Leave empty to use username')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='password'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Password')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='password'
                          placeholder={
                            isUpdate
                              ? t('Leave empty to keep unchanged')
                              : t('Enter password (8–128 characters)')
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SideDrawerSection>

              {/* Group & Quota Settings (Update only) */}
              {isUpdate && (
                <SideDrawerSection>
                  <h3 className='text-sm font-medium'>{t('Group & Quota')}</h3>

                  <FormField
                    control={form.control}
                    name='group'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Group')}</FormLabel>
                        <FormControl>
                          <Combobox
                            options={groups.map((group) => ({
                              value: group,
                              label: group,
                            }))}
                            onValueChange={field.onChange}
                            value={field.value}
                            className='w-full'
                            placeholder={t('Select a group')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='quota_dollars'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Remaining Quota ({{currency}})', {
                            currency: currencyLabel,
                          })}
                        </FormLabel>
                        <div className='flex gap-2'>
                          <FormControl>
                            <Input
                              value={
                                tokensOnly
                                  ? String(field.value || 0)
                                  : (field.value || 0).toFixed(6)
                              }
                              readOnly
                              className='flex-1'
                            />
                          </FormControl>
                          <Button
                            type='button'
                            variant='outline'
                            onClick={() => {
                              setQuotaDialogTargetType('quota')
                              setQuotaDialogOpen(true)
                            }}
                          >
                            <Pencil className='mr-1 h-4 w-4' />
                            {t('Adjust Quota')}
                          </Button>
                        </div>
                        <FormDescription>
                          {formatQuota(parseQuotaFromDollars(field.value || 0))}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='remark'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Remark')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            placeholder={t(
                              'Admin notes (only visible to admins)'
                            )}
                            rows={3}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SideDrawerSection>
              )}

              {/* NovelAI Quota & Allocation (Update only) */}
              {isUpdate && (
                <SideDrawerSection>
                  <div className='flex items-center gap-2'>
                    <Zap className='h-4 w-4 text-amber-500' />
                    <h3 className='text-sm font-medium'>
                      NovelAI 配额与自动分配 (Power / Anlas)
                    </h3>
                  </div>

                  <div className='grid grid-cols-2 gap-4'>
                    <FormField
                      control={form.control}
                      name='power'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel className='flex items-center justify-between'>
                            <span className='flex items-center gap-1'>
                              <Zap className='h-3.5 w-3.5 text-amber-500' />
                              <span>每日电量 (Power 余额)</span>
                            </span>
                            <Button
                              type='button'
                              variant='ghost'
                              size='sm'
                              className='h-5 text-xs text-amber-600 dark:text-amber-400 px-1'
                              onClick={() => {
                                setQuotaDialogTargetType('power')
                                setQuotaDialogOpen(true)
                              }}
                            >
                              ⚡ 调整
                            </Button>
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              {...field}
                              value={field.value ?? 0}
                              onChange={(e) =>
                                field.onChange(parseInt(e.target.value, 10) || 0)
                              }
                            />
                          </FormControl>
                          <FormDescription className='text-xs'>
                            用于 V5 电池电量 (允许负数)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='anlas'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel className='flex items-center justify-between'>
                            <span className='flex items-center gap-1'>
                              <Gem className='h-3.5 w-3.5 text-cyan-500' />
                              <span>Opus 点数 (Anlas 余额)</span>
                            </span>
                            <Button
                              type='button'
                              variant='ghost'
                              size='sm'
                              className='h-5 text-xs text-cyan-600 dark:text-cyan-400 px-1'
                              onClick={() => {
                                setQuotaDialogTargetType('anlas')
                                setQuotaDialogOpen(true)
                              }}
                            >
                              💎 调整
                            </Button>
                          </FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              {...field}
                              value={field.value ?? 0}
                              onChange={(e) =>
                                field.onChange(parseInt(e.target.value, 10) || 0)
                              }
                            />
                          </FormControl>
                          <FormDescription className='text-xs'>
                            用于高规格 Anlas (允许负数)
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  {/* Auto Allocation Settings */}
                  <div className='space-y-3 rounded-lg border p-3.5 bg-muted/20'>
                    <div className='text-xs font-semibold text-muted-foreground uppercase tracking-wider'>
                      自动分配策略 (Auto Allocation)
                    </div>

                    <div className='space-y-2 rounded-md border p-3 bg-background'>
                      <FormField
                        control={form.control}
                        name='auto_power_enabled'
                        render={({ field }) => (
                          <FormItem className='flex flex-row items-center justify-between space-y-0'>
                            <div className='space-y-0.5'>
                              <FormLabel className='text-sm font-medium'>
                                ⚡ 每日自动刷新电量
                              </FormLabel>
                              <FormDescription className='text-xs'>
                                每日定时自动重置该用户电量至设定值
                              </FormDescription>
                            </div>
                            <FormControl>
                              <Checkbox
                                checked={!!field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />

                      {form.watch('auto_power_enabled') && (
                        <FormField
                          control={form.control}
                          name='auto_power_amount'
                          render={({ field }) => (
                            <FormItem className='pt-2 border-t'>
                              <FormLabel className='text-xs'>
                                每日刷新目标电量 (Power)
                              </FormLabel>
                              <FormControl>
                                <Input
                                  type='number'
                                  min={0}
                                  {...field}
                                  value={field.value ?? 50}
                                  onChange={(e) =>
                                    field.onChange(
                                      parseInt(e.target.value, 10) || 0
                                    )
                                  }
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      )}
                    </div>

                    <div className='space-y-2 rounded-md border p-3 bg-background'>
                      <FormField
                        control={form.control}
                        name='auto_anlas_enabled'
                        render={({ field }) => (
                          <FormItem className='flex flex-row items-center justify-between space-y-0'>
                            <div className='space-y-0.5'>
                              <FormLabel className='text-sm font-medium'>
                                💎 每月自动分配 Anlas
                              </FormLabel>
                              <FormDescription className='text-xs'>
                                每月 1 日自动重置/充值该用户 Anlas
                              </FormDescription>
                            </div>
                            <FormControl>
                              <Checkbox
                                checked={!!field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                          </FormItem>
                        )}
                      />

                      {form.watch('auto_anlas_enabled') && (
                        <FormField
                          control={form.control}
                          name='auto_anlas_amount'
                          render={({ field }) => (
                            <FormItem className='pt-2 border-t'>
                              <FormLabel className='text-xs'>
                                每月分配目标 Anlas
                              </FormLabel>
                              <FormControl>
                                <Input
                                  type='number'
                                  min={0}
                                  {...field}
                                  value={field.value ?? 10000}
                                  onChange={(e) =>
                                    field.onChange(
                                      parseInt(e.target.value, 10) || 0
                                    )
                                  }
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      )}
                    </div>
                  </div>
                </SideDrawerSection>
              )}

              {canEditAdminPermissions &&
                targetIsAdmin &&
                permissionCatalog.resources.length > 0 && (
                  <SideDrawerSection>
                    <h3 className='text-sm font-medium'>
                      {t('Admin Permissions')}
                    </h3>
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Default administrator permissions can be overridden for this user.'
                      )}
                    </p>
                    <FormField
                      control={form.control}
                      name='admin_permissions'
                      render={({ field }) => {
                        const selected = normalizeAdminPermissions(
                          field.value,
                          permissionCatalog
                        )
                        return (
                          <FormItem>
                            <div className='space-y-3'>
                              {permissionCatalog.resources.map((resource) => (
                                <div
                                  key={resource.resource}
                                  className='space-y-2 rounded-md border p-3'
                                >
                                  <div className='text-sm font-medium'>
                                    {t(resource.label_key)}
                                  </div>
                                  <div className='space-y-2'>
                                    {resource.actions.map((option) => (
                                      <label
                                        key={option.action}
                                        className='flex items-start gap-3'
                                      >
                                        <Checkbox
                                          checked={
                                            selected[resource.resource]?.[
                                              option.action
                                            ] === true
                                          }
                                          onCheckedChange={(checked) => {
                                            field.onChange({
                                              ...selected,
                                              [resource.resource]: {
                                                ...selected[resource.resource],
                                                [option.action]:
                                                  checked === true,
                                              },
                                            })
                                          }}
                                        />
                                        <span className='flex flex-col gap-1'>
                                          <span className='text-sm font-medium'>
                                            {t(option.label_key)}
                                          </span>
                                          <span className='text-muted-foreground text-xs'>
                                            {t(option.description_key)}
                                          </span>
                                        </span>
                                      </label>
                                    ))}
                                  </div>
                                </div>
                              ))}
                            </div>
                            <FormMessage />
                          </FormItem>
                        )
                      }}
                    />
                    {currentUser && (
                      <p className='text-muted-foreground text-xs'>
                        {hasPermission(
                          currentUser,
                          ADMIN_PERMISSION_RESOURCES.CHANNEL,
                          ADMIN_PERMISSION_ACTIONS.SENSITIVE_WRITE
                        )
                          ? t(
                              'Your account can edit sensitive channel settings.'
                            )
                          : t(
                              'Your account cannot edit sensitive channel settings.'
                            )}
                      </p>
                    )}
                  </SideDrawerSection>
                )}

              {/* Binding Information (Read-only) */}
              {isUpdate && (
                <SideDrawerSection>
                  <h3 className='text-sm font-medium'>
                    {t('Binding Information')}
                  </h3>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Third-party account bindings (read-only, managed by user in Security & Access)'
                    )}
                  </p>

                  <div className='flex flex-col gap-3'>
                    {BINDING_FIELDS.map(({ key, label }) => (
                      <div key={key}>
                        <Label className='text-muted-foreground text-xs'>
                          {t(label)}
                        </Label>
                        <Input
                          value={
                            (currentRow?.[key as keyof User] as string) || '-'
                          }
                          disabled
                          className='mt-1'
                        />
                      </div>
                    ))}
                  </div>
                </SideDrawerSection>
              )}
            </form>
          </Form>
          <SheetFooter className={sideDrawerFooterClassName()}>
            <SheetClose render={<Button variant='outline' />}>
              {t('Close')}
            </SheetClose>
            <Button form='user-form' type='submit' disabled={isSubmitting}>
              {isSubmitting ? t('Saving...') : t('Save changes')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* Adjust Quota Dialog */}
      {currentRow && (
        <UserQuotaDialog
          open={quotaDialogOpen}
          onOpenChange={setQuotaDialogOpen}
          userId={currentRow.id}
          currentQuota={parseQuotaFromDollars(currentQuotaRaw || 0)}
          currentPower={form.watch('power') ?? currentRow.power ?? 0}
          currentAnlas={form.watch('anlas') ?? currentRow.anlas ?? 0}
          initialTargetType={quotaDialogTargetType}
          onSuccess={refreshUserData}
        />
      )}
    </>
  )
}
