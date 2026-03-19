import type { CollabAnnotationMessage } from '@/lib/pdf_collab'

export type AnnotationTool = 'select' | 'textbox' | 'draw'

export type CanvasPathCommand = [string, ...number[]]

type TextAnnotationPayload = {
  text: string
  width?: number
  fontSize?: number
  fill?: string
  angle?: number
}

type CanvasAnnotationPayload = {
  path: CanvasPathCommand[]
  stroke?: string
  strokeWidth?: number
}

export type PDFTextboxAnnotation = {
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

export type PDFCanvasAnnotation = {
  id: number
  type: 'CANVAS'
  page: number
  positionX: number
  positionY: number
  path: CanvasPathCommand[]
  stroke: string
  strokeWidth: number
  createdAt: number
  updatedAt: number
}

export type PDFAnnotation = PDFTextboxAnnotation | PDFCanvasAnnotation

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
  {
    id: 'draw',
    label: 'Draw',
    description: 'Sketch directly on the current page.',
  },
]

const DEFAULT_TEXTBOX_WIDTH = 0.3
const DEFAULT_TEXTBOX_FONT_SIZE = 0.032
const DEFAULT_TEXTBOX_FILL = '#111827'
const DEFAULT_CANVAS_STROKE = '#ef4444'
const DEFAULT_CANVAS_STROKE_WIDTH = 0.004

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

export function createCanvasAnnotation(input: {
  id: number
  page: number
  positionX: number
  positionY: number
  path: CanvasPathCommand[]
  stroke?: string
  strokeWidth?: number
  createdAt?: number
  updatedAt?: number
}): PDFCanvasAnnotation {
  return {
    id: input.id,
    type: 'CANVAS',
    page: input.page,
    positionX: input.positionX,
    positionY: input.positionY,
    path: input.path.map((command) => [...command] as CanvasPathCommand),
    stroke: input.stroke ?? DEFAULT_CANVAS_STROKE,
    strokeWidth: input.strokeWidth ?? DEFAULT_CANVAS_STROKE_WIDTH,
    createdAt: input.createdAt ?? 0,
    updatedAt: input.updatedAt ?? 0,
  }
}

export function fromCollabAnnotation(annotation: CollabAnnotationMessage): PDFAnnotation | null {
  switch (annotation.type) {
    case 'TEXTBOX':
    case 'NOTE': {
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

    case 'CANVAS': {
      const payload = parseCanvasAnnotationData(annotation.data)
      if (!payload.path.length) {
        return null
      }

      return createCanvasAnnotation({
        id: annotation.id,
        page: annotation.page,
        positionX: annotation.positionX,
        positionY: annotation.positionY,
        path: payload.path,
        stroke: payload.stroke,
        strokeWidth: payload.strokeWidth,
        createdAt: annotation.createdAt,
        updatedAt: annotation.updatedAt,
      })
    }

    default:
      return null
  }
}

export function toCollabAnnotation(annotation: PDFAnnotation): CollabAnnotationMessage {
  if (annotation.type === 'CANVAS') {
    return {
      id: annotation.id,
      type: annotation.type,
      data: JSON.stringify({
        path: annotation.path,
        stroke: annotation.stroke,
        strokeWidth: annotation.strokeWidth,
      }),
      page: annotation.page,
      createdAt: annotation.createdAt,
      updatedAt: annotation.updatedAt,
      positionX: annotation.positionX,
      positionY: annotation.positionY,
    }
  }

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
  if (annotation.type === 'CANVAS') {
    return {
      ...annotation,
      path: annotation.path.map((command) => [...command] as CanvasPathCommand),
    }
  }

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

function parseCanvasAnnotationData(data: string): CanvasAnnotationPayload {
  if (!data) {
    return { path: [] }
  }

  try {
    const parsed = JSON.parse(data) as Partial<CanvasAnnotationPayload>
    if (parsed && typeof parsed === 'object') {
      const path = Array.isArray(parsed.path)
        ? parsed.path
          .map((command) => normalizeCanvasPathCommand(command))
          .filter((command): command is CanvasPathCommand => command !== null)
        : []

      return {
        path,
        stroke: typeof parsed.stroke === 'string' ? parsed.stroke : undefined,
        strokeWidth: typeof parsed.strokeWidth === 'number' ? parsed.strokeWidth : undefined,
      }
    }
  } catch {
  }

  return { path: [] }
}

function normalizeCanvasPathCommand(command: unknown): CanvasPathCommand | null {
  if (!Array.isArray(command) || typeof command[0] !== 'string') {
    return null
  }

  const numericArgs = command.slice(1).every((value) => typeof value === 'number')
  if (!numericArgs) {
    return null
  }

  return [command[0], ...(command.slice(1) as number[])]
}
