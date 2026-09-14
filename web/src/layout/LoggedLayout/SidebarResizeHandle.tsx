import { useEffect, useRef } from 'react'

import {
  clampSidebarWidth,
  SIDEBAR_WIDTH_DEFAULT,
  SIDEBAR_WIDTH_MAX,
  SIDEBAR_WIDTH_MIN,
} from '../../config'

import { saveSidebarWidth } from './sidebarWidth'
import styles from './styles.module.scss'

type Props = {
  width: number
  onWidthChange: (width: number) => void
}

export function SidebarResizeHandle({ width, onWidthChange }: Props) {
  const draggingRef = useRef(false)
  const startXRef = useRef(0)
  const startWidthRef = useRef(width)
  const widthRef = useRef(width)

  useEffect(() => {
    widthRef.current = width
  }, [width])

  useEffect(() => {
    const onMove = (e: PointerEvent) => {
      if (!draggingRef.current) return
      onWidthChange(clampSidebarWidth(startWidthRef.current + (e.clientX - startXRef.current)))
    }
    const onUp = () => {
      if (!draggingRef.current) return
      draggingRef.current = false
      document.body.classList.remove('is-sidebar-resizing')
      saveSidebarWidth(clampSidebarWidth(widthRef.current))
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
    window.addEventListener('pointercancel', onUp)
    return () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
      window.removeEventListener('pointercancel', onUp)
      document.body.classList.remove('is-sidebar-resizing')
    }
  }, [onWidthChange])

  return (
    <div
      className={styles.sidebarResizer}
      role="separator"
      aria-orientation="vertical"
      aria-label="调整侧边栏宽度"
      aria-valuemin={SIDEBAR_WIDTH_MIN}
      aria-valuemax={SIDEBAR_WIDTH_MAX}
      aria-valuenow={width}
      title="拖动调整侧边栏宽度（双击复位）"
      onPointerDown={(e) => {
        if (e.button !== 0) return
        e.preventDefault()
        draggingRef.current = true
        startXRef.current = e.clientX
        startWidthRef.current = widthRef.current
        document.body.classList.add('is-sidebar-resizing')
      }}
      onDoubleClick={() => {
        onWidthChange(SIDEBAR_WIDTH_DEFAULT)
        saveSidebarWidth(SIDEBAR_WIDTH_DEFAULT)
      }}
    />
  )
}
