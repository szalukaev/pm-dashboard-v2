This directory should contain the invoice_template.docx file.

The template is a Word document (.docx) that contains placeholder text
that gets replaced during invoice generation:

Placeholders:
  {{company}}      - Company name
  {{address}}      - Company address
  {{phone}}        - Contact phone
  {{contact}}      - Contact name
  {{invoice_num}}  - Invoice number (YYYYMM-DD format)
  {{date}}         - Invoice date (YYYY-MM-DD)
  {{vat_label}}    - "НДС 5%" or "Без НДС"
  {{item_name}}    - First item name
  {{item_qty}}     - First item quantity
  {{item_price}}   - First item price
  {{item_amount}}  - First item amount
  {{subtotal}}     - Subtotal (without VAT)
  {{vat_amount}}   - VAT amount
  {{total}}        - Total (with VAT)

The template file must be a valid .docx (ZIP archive with word/document.xml).
Create it in Word/LibreOffice, add the placeholders above as regular text,
and save as .docx.
