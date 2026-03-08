/*
 * Copyright 2026 Benjamin Touchard (kOlapsis)
 *
 * Licensed under the GNU Affero General Public License v3.0 (AGPL-3.0)
 * or a commercial license. You may not use this file except in compliance
 * with one of these licenses.
 *
 * AGPL-3.0: https://www.gnu.org/licenses/agpl-3.0.html
 * Commercial: See COMMERCIAL-LICENSE.md
 *
 * Source: https://github.com/kolapsis/maintenant
 */

export interface StateStyle {
  color: string
  bg: string
  glow?: string
}

const stateStyles: Record<string, StateStyle> = {
  running: { color: 'var(--pb-status-ok)', bg: 'var(--pb-status-ok-bg)', glow: 'var(--pb-glow-ok)' },
  exited: { color: 'var(--pb-status-down)', bg: 'var(--pb-status-down-bg)' },
  completed: { color: 'var(--pb-text-secondary)', bg: 'var(--pb-bg-elevated)' },
  restarting: { color: 'var(--pb-status-warn)', bg: 'var(--pb-status-warn-bg)', glow: 'var(--pb-glow-warn)' },
  paused: { color: 'var(--pb-accent)', bg: 'var(--pb-bg-elevated)' },
  created: { color: 'var(--pb-text-muted)', bg: 'var(--pb-bg-elevated)' },
  dead: { color: 'var(--pb-status-down)', bg: 'var(--pb-status-down-bg)' },
}

const defaultStyle: StateStyle = { color: 'var(--pb-text-muted)', bg: 'var(--pb-bg-elevated)' }

export function getStateStyle(state: string): StateStyle {
  return stateStyles[state] ?? defaultStyle
}

export function getStateColor(state: string): string {
  return (stateStyles[state] ?? defaultStyle).color
}

/** Exit codes that indicate a graceful/voluntary stop, not a crash. */
const gracefulExitCodes = new Set([0, 137, 143])

export function isGracefulExitCode(code: number): boolean {
  return gracefulExitCodes.has(code)
}

export interface ExitCodeStyle {
  bg: string
  color: string
}

export function getExitCodeStyle(code: number): ExitCodeStyle {
  if (isGracefulExitCode(code)) {
    return { bg: 'var(--pb-bg-elevated)', color: 'var(--pb-text-secondary)' }
  }
  return { bg: 'var(--pb-status-down-bg)', color: 'var(--pb-status-down)' }
}
