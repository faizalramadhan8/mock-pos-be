-- Rename kategori "Beban Pegawai" → "Gaji & Lemburan Pegawai" supaya lebih
-- self-explanatory untuk Bu Santi (non-akuntan). "Beban Pegawai" istilah
-- akuntansi formal yang ambiguous (gaji? bonus? lemburan?). Label baru
-- eksplisit mencakup semua komponen pembayaran ke pegawai.

UPDATE expense_categories
SET name = 'Gaji & Lemburan Pegawai'
WHERE name = 'Beban Pegawai' AND is_system = 1;
