-- Rollback: hapus 12 kategori seeded. WHERE by nama exact match — kalau
-- nanti nama di-rename via admin UI, rollback tidak hapus (safe by design).
DELETE FROM ecom_categories WHERE name IN (
  'Flour & Dry Goods',
  'Chocolate & Compound',
  'Butter & Margarine',
  'Milk & Cream',
  'Cheese',
  'Filling & Topping',
  'Sprinkles & Decor',
  'Flavoring & Coloring',
  'Leavening & Emulsifier',
  'Sugar & Sweetener',
  'Packaging & Tools',
  'Frozen & Ready-to-eat'
);
