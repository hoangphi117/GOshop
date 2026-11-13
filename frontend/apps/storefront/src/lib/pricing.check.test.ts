import assert from "node:assert/strict";
import { getDiscount } from "./pricing.ts";

const base = { uuid: "u", name: "n", price: 1_000_000 } as never;

assert.equal(getDiscount({ ...(base as object), price: 800_000, compare_at_price: 1_000_000 } as never)?.percent, 20);
assert.equal(getDiscount({ ...(base as object), price: 800_000 } as never), null, "no compare_at_price = no discount");
assert.equal(getDiscount({ ...(base as object), price: 800_000, compare_at_price: 700_000 } as never), null, "compare below price = no discount");

console.log("pricing ok");
