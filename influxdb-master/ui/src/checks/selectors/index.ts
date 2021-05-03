import {AppState, Check} from 'src/types'

export const getCheck = (state: AppState, id: string): Check => {
  return state.resources.checks.byID[id] || null
}

export const getCheckIDs = (state: AppState): {[x: string]: boolean} => {
  return state.resources.checks.allIDs.reduce(
    (acc, id) => ({...acc, [id]: true}),
    {}
  )
}

export const sortChecksByName = (checks: Check[]): Check[] =>
  checks.sort((a, b) => (a.name.toLowerCase() > b.name.toLowerCase() ? 1 : -1))
