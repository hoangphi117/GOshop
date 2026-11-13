import type { Product } from "@/lib/api-server";

/**
 * The discount a product actually advertises.
 *
 * Only compare_at_price counts. Inventing a higher "original" price makes the
 * struck-through number a lie, which is illegal advertising in most markets and
 * the first thing a buyer checks.
 */
export function getDiscount(product: Product): { original: number; percent: number } | null {
  const compare = product.compare_at_price;
  if (!compare || compare <= product.price) return null;
  return {
    original: compare,
    percent: Math.round(((compare - product.price) / compare) * 100),
  };
}
