import { useEffect, useLayoutEffect, useRef, type RefObject } from "react";

/**
 * useFocusLeaving has left give the focus elsewhere as the element of ref
 * leaves the document holding it (v0.1 design 13.2, item 17): before it
 * goes, in a layout effect's cleanup, so that the focus does not fall to
 * the page's start. left is the latest given.
 */
export function useFocusLeaving(ref: RefObject<HTMLElement | null>, left: () => void): void {
  const leaving = useRef(left);
  useEffect(() => {
    leaving.current = left;
  });
  useLayoutEffect(() => {
    const element = ref.current;
    return () => {
      if (element?.contains(document.activeElement)) {
        leaving.current();
      }
    };
  }, [ref]);
}
