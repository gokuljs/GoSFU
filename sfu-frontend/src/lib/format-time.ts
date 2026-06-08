const HAS_TIMEZONE_SUFFIX = /(?:[zZ]|[+-]\d{2}:\d{2})$/

function browserTimeZone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone
}

const localTimeFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
  timeZone: browserTimeZone(),
})

/** Parse RFC3339/ISO timestamps as UTC when no offset is present. */
export function parseUtcTimestamp(value: string): Date {
  const trimmed = value.trim()
  if (!trimmed) return new Date(Number.NaN)

  if (HAS_TIMEZONE_SUFFIX.test(trimmed)) {
    return new Date(trimmed)
  }

  if (trimmed.includes("T")) {
    return new Date(`${trimmed}Z`)
  }

  return new Date(`${trimmed}T00:00:00Z`)
}

/** Format a UTC timestamp for display in the browser's local timezone. */
export function formatLocalTime(value: string) {
  const date = parseUtcTimestamp(value)
  if (Number.isNaN(date.getTime())) return "--:--:--"
  return localTimeFormatter.format(date)
}
