// useForm — hook minimalista de formulário com validação Zod
// Evita dependência extra (react-hook-form) mantendo o foco na funcionalidade
import { useState, useCallback, useRef } from 'react';
import type { z } from 'zod';

type Errors<T> = Partial<Record<keyof T, string>>;

interface UseFormOptions<T extends Record<string, unknown>> {
  schema: z.ZodType<T>;
  defaultValues: T;
}

interface UseFormReturn<T extends Record<string, unknown>> {
  values: T;
  errors: Errors<T>;
  register: (field: keyof T) => {
    name: string;
    value: string;
    onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => void;
    onBlur: () => void;
  };
  handleSubmit: (onValid: (data: T) => void | Promise<void>) => (e: React.FormEvent) => void;
  setValue: (field: keyof T, value: unknown) => void;
  reset: (values?: Partial<T>) => void;
  setError: (field: keyof T, message: string) => void;
  clearErrors: () => void;
  isSubmitting: boolean;
}

export function useForm<T extends Record<string, unknown>>({
  schema,
  defaultValues,
}: UseFormOptions<T>): UseFormReturn<T> {
  const [values, setValues] = useState<T>(defaultValues);
  const [errors, setErrors] = useState<Errors<T>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const defaultsRef = useRef(defaultValues);

  const register = useCallback((field: keyof T) => ({
    name: String(field),
    value: String(values[field] ?? ''),
    onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
      setValues(prev => ({ ...prev, [field]: e.target.value }));
      // Limpar erro ao editar
      setErrors(prev => ({ ...prev, [field]: undefined }));
    },
    onBlur: () => {
      // Validação por campo no blur
      const result = schema.safeParse(values);
      if (!result.success) {
        const fieldError = result.error.flatten().fieldErrors[field as string];
        if (fieldError?.[0]) {
          setErrors(prev => ({ ...prev, [field]: fieldError[0] }));
        }
      }
    },
  }), [values, schema]);

  const setValue = useCallback((field: keyof T, value: unknown) => {
    setValues(prev => ({ ...prev, [field]: value }));
  }, []);

  const reset = useCallback((newValues?: Partial<T>) => {
    setValues({ ...defaultsRef.current, ...newValues });
    setErrors({});
  }, []);

  const setError = useCallback((field: keyof T, message: string) => {
    setErrors(prev => ({ ...prev, [field]: message }));
  }, []);

  const clearErrors = useCallback(() => setErrors({}), []);

  const handleSubmit = useCallback(
    (onValid: (data: T) => void | Promise<void>) =>
      (e: React.FormEvent) => {
        e.preventDefault();
        const result = schema.safeParse(values);
        if (!result.success) {
          const fieldErrors: Errors<T> = {};
          for (const [key, msgs] of Object.entries(result.error.flatten().fieldErrors)) {
            fieldErrors[key as keyof T] = (msgs as string[])[0];
          }
          setErrors(fieldErrors);
          return;
        }
        setIsSubmitting(true);
        Promise.resolve(onValid(result.data)).finally(() => setIsSubmitting(false));
      },
    [values, schema],
  );

  return { values, errors, register, handleSubmit, setValue, reset, setError, clearErrors, isSubmitting };
}
