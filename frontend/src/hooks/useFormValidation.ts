import { useState, useCallback } from 'react'

// 单条字段校验规则
export interface FieldRule {
  // 字段唯一标识，用于标红定位
  field: string
  // 多步骤表单中字段所属步骤索引（单步骤表单可不填）
  step?: number
  // 校验函数：返回 true 表示该字段有效
  valid: () => boolean
}

export interface ValidateResult {
  ok: boolean
  // 第一个出错字段所在步骤（多步骤表单用于跳转）
  firstErrorStep?: number
  // 第一个出错字段标识
  firstErrorField?: string
}

/**
 * 通用表单完整性校验 hook。
 * 设计目标：
 * 1. 仅在用户点击保存时执行校验，打开表单不会预先标红。
 * 2. 校验失败返回首个错误字段及其所属步骤，便于跳转并标红。
 * 3. 用户修改某字段后通过 clearError 即时清除该字段红框。
 */
export function useFormValidation() {
  const [errors, setErrors] = useState<Set<string>>(new Set())

  // 执行校验，按规则顺序找出全部错误字段
  const validate = useCallback((rules: FieldRule[]): ValidateResult => {
    const next = new Set<string>()
    let firstErrorStep: number | undefined
    let firstErrorField: string | undefined
    for (const rule of rules) {
      if (!rule.valid()) {
        next.add(rule.field)
        if (firstErrorField === undefined) {
          firstErrorField = rule.field
          firstErrorStep = rule.step
        }
      }
    }
    setErrors(next)
    return { ok: next.size === 0, firstErrorStep, firstErrorField }
  }, [])

  // 清除单个字段的错误态（用户开始修改时调用）
  const clearError = useCallback((field: string) => {
    setErrors((prev) => {
      if (!prev.has(field)) return prev
      const next = new Set(prev)
      next.delete(field)
      return next
    })
  }, [])

  // 重置全部错误态（打开/关闭表单时调用）
  const reset = useCallback(() => {
    setErrors(new Set())
  }, [])

  // 判断字段是否处于错误态
  const hasError = useCallback((field: string) => errors.has(field), [errors])

  return { errors, validate, clearError, reset, hasError }
}
