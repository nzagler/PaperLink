import type { CollabAnnotationMessage } from '@/lib/pdf_collab'

export type AnnotationTool = 'select' | 'textbox'

type TextAnnotationPayload = {
  text: string
  width?: number
  fontSize?: number
  fill?: string
  angle?: number
}

export type PDFAnnotation = {
  id: number
  type: 'TEXTBOX'
  page: number
  text: string
  positionX: number
  positionY: number
  width: number
  fontSize: number
  fill: string
  angle: number
  createdAt: number
  updatedAt: number
}

export const annotationTools: Array<{
  id: AnnotationTool
  label: string
  description: string
}> = [
  {
    id: 'select',
    label: 'Select',
    description: 'Move and edit existing annotations.',
  },
  {
    id: 'textbox',
    label: 'Text Box',
    description: 'Create a text annotation on the current page.',
  },
]

const DEFAULT_TEXTBOX_WIDTH = 0.3
const DEFAULT_TEXTBOX_FONT_SIZE = 0.032
const DEFAULT_TEXTBOX_FILL = '#111827'

export function createTextboxAnnotation(input: {
  id: number
  page: number
  text: string
  positionX: number
  positionY: number
  width?: number
  fontSize?: number
  fill?: string
  angle?: number
  createdAt?: number
  updatedAt?: number
}): PDFAnnotation {
  return {
    id: input.id,
    type: 'TEXTBOX',
    page: input.page,
    text: input.text,
    positionX: input.positionX,
    positionY: input.positionY,
    width: input.width ?? DEFAULT_TEXTBOX_WIDTH,
    fontSize: input.fontSize ?? DEFAULT_TEXTBOX_FONT_SIZE,
    fill: input.fill ?? DEFAULT_TEXTBOX_FILL,
    angle: input.angle ?? 0,
    createdAt: input.createdAt ?? 0,
    updatedAt: input.updatedAt ?? 0,
  }
}

export function fromCollabAnnotation(annotation: CollabAnnotationMessage): PDFAnnotation | null {
  if (annotation.type !== 'TEXTBOX' && annotation.type !== 'NOTE') {
    return null
  }

  const payload = parseTextAnnotationData(annotation.data)
  return createTextboxAnnotation({
    id: annotation.id,
    page: annotation.page,
    text: payload.text,
    positionX: annotation.positionX,
    positionY: annotation.positionY,
    width: payload.width,
    fontSize: payload.fontSize,
    fill: payload.fill,
    angle: payload.angle,
    createdAt: annotation.createdAt,
    updatedAt: annotation.updatedAt,
  })
}

export function toCollabAnnotation(annotation: PDFAnnotation): CollabAnnotationMessage {
  return {
    id: annotation.id,
    type: annotation.type,
    data: JSON.stringify({
      text: annotation.text,
      width: annotation.width,
      fontSize: annotation.fontSize,
      fill: annotation.fill,
      angle: annotation.angle,
    }),
    page: annotation.page,
    createdAt: annotation.createdAt,
    updatedAt: annotation.updatedAt,
    positionX: annotation.positionX,
    positionY: annotation.positionY,
  }
}

export function cloneAnnotation(annotation: PDFAnnotation): PDFAnnotation {
  return { ...annotation }
}

function parseTextAnnotationData(data: string): TextAnnotationPayload {
  if (!data) {
    return { text: '' }
  }

  try {
    const parsed = JSON.parse(data) as Partial<TextAnnotationPayload>
    if (parsed && typeof parsed === 'object') {
      return {
        text: typeof parsed.text === 'string' ? parsed.text : '',
        width: typeof parsed.width === 'number' ? parsed.width : undefined,
        fontSize: typeof parsed.fontSize === 'number' ? parsed.fontSize : undefined,
        fill: typeof parsed.fill === 'string' ? parsed.fill : undefined,
        angle: typeof parsed.angle === 'number' ? parsed.angle : undefined,
      }
    }
  } catch {
  }

  return {
    text: data,
  }
}
