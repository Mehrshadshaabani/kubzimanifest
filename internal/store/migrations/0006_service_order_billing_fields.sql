-- Billing-address fields a real card payment processor's checkout
-- typically requires (name/email were already collected) — phone, full
-- address, and postal code.
ALTER TABLE service_orders ADD COLUMN IF NOT EXISTS contact_phone TEXT NOT NULL DEFAULT '';
ALTER TABLE service_orders ADD COLUMN IF NOT EXISTS contact_address TEXT NOT NULL DEFAULT '';
ALTER TABLE service_orders ADD COLUMN IF NOT EXISTS contact_postal_code TEXT NOT NULL DEFAULT '';
