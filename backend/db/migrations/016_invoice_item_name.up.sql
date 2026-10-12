-- The name of a line of an invoice is free text: a reference to a contract
-- and its appendix easily takes more than the 500 characters it was given.
ALTER TABLE invoice_items ALTER COLUMN name TYPE TEXT;
