invoice_template.docx is the built-in template of invoices. It is used
until the administrator uploads another one (Settings → Administration →
Invoice template); the uploaded file is kept in the data directory.

A template is an ordinary Word document. What changes from invoice to
invoice is written as variables — {{company}}, {{total}} and so on; the
full list is in data.go and on the screen of the administrator.

The lines of an invoice go into the template in one of two ways:
  - {{items_table}} alone on a line is replaced by a ready table;
  - a table row with item variables ({{item_name}}, {{item_amount}}, …) is
    repeated for every line and keeps its formatting.

generate_template.py builds the built-in template.
