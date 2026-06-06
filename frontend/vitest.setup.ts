import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { mutate } from "swr";

// Clear SWR's global cache between tests so cached data from one test never
// leaks into the next (each test starts from a clean, loading state).
afterEach(() => {
  mutate(() => true, undefined, { revalidate: false });
});
