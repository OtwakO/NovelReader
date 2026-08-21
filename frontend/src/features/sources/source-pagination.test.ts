import { describe, expect, test } from "vitest";
import {
  clampPage,
  pageCount,
  pageItems,
  pageRange,
} from "./source-pagination";

const sources = Array.from({ length: 58 }, (_, index) => `source-${index + 1}`);

describe("BookSource pagination", () => {
  test("renders only the requested page", () => {
    expect(pageItems(sources, 2, 25)).toEqual(sources.slice(25, 50));
    expect(pageCount(sources.length, 25)).toBe(3);
    expect(pageRange(2, 25, sources.length)).toEqual({ start: 26, end: 50 });
  });

  test("clamps pages after filters or deletion reduce the result set", () => {
    expect(clampPage(5, 58, 25)).toBe(3);
    expect(clampPage(3, 12, 25)).toBe(1);
    expect(pageRange(1, 25, 0)).toEqual({ start: 0, end: 0 });
  });
});
