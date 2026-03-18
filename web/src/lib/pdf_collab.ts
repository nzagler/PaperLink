export type CollabStatus = 'idle' | 'connecting' | 'connected' | 'disconnected'

export type CollabUser = {
  userId: number
  username: string
}

export type CollabAnnotationType = 'TEXTBOX' | 'NOTE' | 'CANVAS'

export type CollabAnnotationMessage = {
  id: number
  type: CollabAnnotationType
  data: string
  page: number
  createdAt: number
  updatedAt: number
  positionX: number
  positionY: number
}

export type CollabServerMessage = {
  type: string
  documentId?: string
  user?: CollabUser
  users?: CollabUser[]
  page?: number
  annotation?: CollabAnnotationMessage
  annotations?: CollabAnnotationMessage[]
  annotationId?: number
  error?: string
}

export type CollabClientMessage = {
  type: string
  page?: number
  annotation?: CollabAnnotationMessage
  annotationId?: number
}
