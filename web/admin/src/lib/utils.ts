import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatUptime(uptime: string): string {
  return uptime || '-'
}

export function formatTime(iso: string): string {
  if (!iso) return '-'
  return new Date(iso).toLocaleString()
}

export function truncateHash(hash: string, len = 16): string {
  if (!hash) return '-'
  return hash.slice(0, len) + '...'
}
