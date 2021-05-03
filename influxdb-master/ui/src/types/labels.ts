import {Label as GLabel} from 'src/client'
import {RemoteDataState} from 'src/types'

// GenLabel is the shape of a Label returned from the server -- before it has
// been modified with UI specific fields
export type GenLabel = GLabel
export interface Label extends GenLabel {
  status: RemoteDataState
  properties: LabelProperties
}

export interface LabelProperties {
  color: string
  description: string
  [k: string]: string
}
