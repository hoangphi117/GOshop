-- Add parent_id and level to categories for Category Tree
ALTER TABLE categories
  ADD COLUMN parent_id INTEGER REFERENCES categories(category_id) ON DELETE SET NULL,
  ADD COLUMN category_level INTEGER NOT NULL DEFAULT 1;

CREATE INDEX idx_categories_parent_id ON categories(parent_id);

-- Add product_specifications JSONB to products for dynamic specifications
ALTER TABLE products
  ADD COLUMN product_specifications JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX idx_products_specifications ON products USING gin(product_specifications);
