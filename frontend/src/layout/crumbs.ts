import type { Params } from 'react-router'

/** Route `handle` shape: the breadcrumb label, static or built from the URL params. */
export interface RouteHandle {
  crumb: string | ((params: Params) => string)
}
