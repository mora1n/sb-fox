import { nextTick } from 'vue'

type ScrollTarget = HTMLElement | null | undefined

type ScrollSnapshot = {
  tops: number[]
  activeElement: HTMLElement | null
}

export function useScrollPreserver(getTargets: () => ScrollTarget[]) {
  let restoreSequence = 0
  let restoreFrame = 0

  function targets() {
    return getTargets().filter((target): target is HTMLElement => target instanceof HTMLElement)
  }

  function capture(): ScrollSnapshot {
    const activeElement = document.activeElement
    return {
      tops: targets().map((target) => target.scrollTop),
      activeElement: activeElement instanceof HTMLElement ? activeElement : null,
    }
  }

  function scheduleRestore(snapshot: ScrollSnapshot) {
    const sequence = ++restoreSequence
    if (restoreFrame) {
      cancelAnimationFrame(restoreFrame)
      restoreFrame = 0
    }
    void nextTick().then(() => {
      if (sequence !== restoreSequence) return
      restoreFrame = requestAnimationFrame(() => {
        restoreFrame = 0
        if (sequence !== restoreSequence) return
        targets().forEach((target, index) => {
          if (snapshot.tops[index] !== undefined) target.scrollTop = snapshot.tops[index]
        })
        if (snapshot.activeElement?.isConnected && document.activeElement !== snapshot.activeElement) {
          snapshot.activeElement.focus({ preventScroll: true })
        }
      })
    })
  }

  function preserveScroll(action: () => void) {
    const snapshot = capture()
    action()
    scheduleRestore(snapshot)
  }

  function preserveScrollAfterUpdate() {
    scheduleRestore(capture())
  }

  return { preserveScroll, preserveScrollAfterUpdate }
}
