import type { Target } from 'mediabunny';

// Target events are notifications: exceptions in listeners are swallowed.
// Cancel synchronously on the first overflow; callers retain the size error
// because execute() may reject with a generic ConversionCanceledError.
export const cancelOnVideoOutputLimit = (
  target: Target, limit: number, cancel: () => void, makeError: () => Error,
): (() => Error | undefined) => {
  let error: Error | undefined;
  target.on('write', ({ end }) => {
    if (end > limit && !error) {
      error = makeError();
      cancel();
    }
  });
  return () => error;
};
