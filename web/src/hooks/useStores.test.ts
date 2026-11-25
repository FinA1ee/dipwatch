import { describe, expect, it } from "vitest";
import { validPick } from "./useStores";

const all = [
  { id: "kakaku", name: "Kakaku.com", region: "JP" },
  { id: "smzdm", name: "SMZDM", region: "CN" },
];

describe("validPick", () => {
  it("keeps a valid pick within the limit", () => {
    expect(validPick(["smzdm"], all, 1)).toEqual(["smzdm"]);
    expect(validPick(["smzdm", "kakaku"], all, 1)).toEqual(["smzdm"]);
    expect(validPick(["smzdm", "kakaku"], all, 2)).toEqual(["smzdm", "kakaku"]);
  });

  it("drops stores that were turned off and falls back to the first", () => {
    expect(validPick(["amazon-us"], all, 1)).toEqual(["kakaku"]);
    expect(validPick([], all, 1)).toEqual(["kakaku"]);
    expect(validPick(["x"], [], 1)).toEqual([]);
  });
});
