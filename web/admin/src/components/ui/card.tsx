import { cn } from '@/lib/utils'

interface CardProps {
  children: React.ReactNode
  className?: string
}

export function Card({ children, className }: CardProps) {
  return (
    <div className={cn('rounded-xl border border-slate-700 bg-slate-800 p-6', className)}>
      {children}
    </div>
  )
}

export function CardHeader({ children, className }: CardProps) {
  return <div className={cn('mb-4', className)}>{children}</div>
}

export function CardTitle({ children, className }: CardProps) {
  return <h3 className={cn('text-sm font-medium uppercase tracking-wider text-slate-400', className)}>{children}</h3>
}

export function CardValue({ children, className }: CardProps) {
  return <div className={cn('text-3xl font-bold text-slate-100', className)}>{children}</div>
}
