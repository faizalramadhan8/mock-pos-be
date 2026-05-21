DELETE FROM expense_categories
WHERE name IN ('Belanja Yoeks', 'Belanja Indogrosir', 'Belanja Indoguna', 'Belanja Sukanda')
  AND is_system = 1;
