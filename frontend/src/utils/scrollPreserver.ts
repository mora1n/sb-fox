import { nextTick } from 'vue'

type ScrollTarget = HTMLElement | null | undefined

type ScrollSnapshot = {
  tops: number[]
  activeElement: HTMLElement | null
}

export function useScrollPreserver(getTargets: () => ScrollTarget[]) {
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

  async function restore(snapshot: ScrollSnapshot) {
    await nextTick()
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    targets().forEach((target, index) => {
      if (snapshot.tops[index] !== undefined) target.scrollTop = snapshot.tops[index]
    })
    if (snapshot.activeElement?.isConnected && document.activeElement !== snapshot.activeElement) {
      snapshot.activeElement.focus({ preventScroll: true })
    }
  }

  function preserveScroll(action: () => void) {
    const snapshot = capture()
    action()
    void restore(snapshot)
  }

  function preserveScrollAfterUpdate() {
    void restore(capture())
  }

  return { preserveScroll, preserveScrollAfterUpdate }
}
