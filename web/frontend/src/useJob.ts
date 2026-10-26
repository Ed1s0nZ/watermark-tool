import { useEffect, useRef, useState } from "react";
export function useJob<T>() {
  const pending = useRef<AbortController | undefined>(undefined);
  const [value, setValue] = useState<T>();
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  const [cancelled, setCancelled] = useState(false);
  function reset() {
    pending.current?.abort();
    pending.current = undefined;
    setBusy(false);
    setValue(undefined);
    setError(undefined);
    setCancelled(false);
  }
  function cancel() {
    reset();
    setCancelled(true);
  }
  useEffect(
    () => () => {
      pending.current?.abort();
      pending.current = undefined;
    },
    [],
  );
  async function run(fn: (signal: AbortSignal) => Promise<T>) {
    if (pending.current) return;
    reset();
    const c = new AbortController();
    pending.current = c;
    setBusy(true);
    try {
      const v = await fn(c.signal);
      if (pending.current === c && !c.signal.aborted) setValue(v);
    } catch (e) {
      if (pending.current === c && !c.signal.aborted) setError(e);
    } finally {
      if (pending.current === c) {
        pending.current = undefined;
        setBusy(false);
      }
    }
  }
  return { value, error, busy, cancelled, reset, cancel, run };
}
