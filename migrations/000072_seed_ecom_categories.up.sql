-- 3 Aug 2026 — Seed 12 kategori ecom customer-facing.
--
-- Latar belakang: Bu Santi punya ~593 produk aktif, semua sudah
-- ecom_is_available=1 tapi ecom_category_id semua NULL. "POS category" di DB
-- sebenarnya = nama supplier (Adyaceda, Heri Jatinegara, Yoeks, dst) — bukan
-- taxonomy produk. Storefront butuh kategori customer-facing baru.
--
-- Design decision (3 Aug 2026): 12 kategori sweet spot. Reasoning:
--   - Grid Home 4 kolom mobile / 6 kolom tablet → 12 = 3 rows / 2 rows clean
--   - < 12 terlalu broad (susah scan), > 12 choice paralysis + beberapa kategori
--     sedikit isi (mis. minuman 5 item, frozen 3 item)
--   - Bu Santi bisa split lagi via admin UI kalau perlu
--
-- Sort order pakai step 10 (10, 20, ..., 120) supaya kalau nanti insert
-- kategori baru bisa isi slot 15, 25 dst tanpa renumber semua.
--
-- Icon dipilih dari CATEGORY_ICONS di bakeshop-ecom/src/components/CategoryIcon.tsx
-- (Lucide icons whitelist). Jangan pakai icon di luar list itu — FE fallback
-- ke Package icon default kalau string tidak match.
--
-- Assignment produk ke kategori TIDAK di-migration ini — Bu Santi manual
-- pilih via /shop/admin/produk → Edit → dropdown Kategori Ecom.

INSERT INTO ecom_categories (id, name, name_id, icon_name, sort_order, is_active) VALUES
  (UUID(), 'Flour & Dry Goods',        'Tepung & Bahan Kering',  'Wheat',     10, 1),
  (UUID(), 'Chocolate & Compound',     'Cokelat & Compound',     'Cookie',    20, 1),
  (UUID(), 'Butter & Margarine',       'Mentega & Margarin',     'Croissant', 30, 1),
  (UUID(), 'Milk & Cream',             'Susu & Cream',           'Milk',      40, 1),
  (UUID(), 'Cheese',                   'Keju',                   'ChefHat',   50, 1),
  (UUID(), 'Filling & Topping',        'Filling & Topping',      'Utensils',  60, 1),
  (UUID(), 'Sprinkles & Decor',        'Meises & Dekorasi',      'Popcorn',   70, 1),
  (UUID(), 'Flavoring & Coloring',     'Perisa & Pewarna',       'Sparkles',  80, 1),
  (UUID(), 'Leavening & Emulsifier',   'Bahan Pengembang',       'Egg',       90, 1),
  (UUID(), 'Sugar & Sweetener',        'Gula & Pemanis',         'Candy',    100, 1),
  (UUID(), 'Packaging & Tools',        'Kemasan & Alat',         'Package',  110, 1),
  (UUID(), 'Frozen & Ready-to-eat',    'Frozen & Siap Saji',     'Sandwich', 120, 1);
