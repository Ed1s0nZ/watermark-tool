"""Reproducible test fixture only; reportlab is not a server dependency."""
from pathlib import Path
from reportlab.pdfgen import canvas
from reportlab.lib.pagesizes import A4

output = Path(__file__).with_name("source.pdf")
if output.exists():
    raise SystemExit("Refusing to overwrite the reference fixture; remove it explicitly to regenerate.")
c = canvas.Canvas(str(output), pagesize=A4, invariant=1, pageCompression=0)
width, height = A4
for page in range(1, 4):
    c.setFillColorRGB(0.08, 0.13, 0.25)
    c.setFont("Helvetica-Bold", 26)
    c.drawString(54, height - 74, f"Page protection fixture - {page}")
    c.setFont("Helvetica", 12)
    c.drawString(54, height - 112, "Original page content must remain visually unchanged.")
    c.setStrokeColorRGB(0.18, 0.35, 0.8)
    c.setLineWidth(2)
    c.line(54, height - 134, width - 54, height - 134)
    c.setFillColorRGB(0.92, 0.95, 1)
    c.roundRect(54, height - 300, width - 108, 132, 12, fill=1, stroke=0)
    c.setFillColorRGB(0.1, 0.2, 0.4)
    c.drawString(74, height - 205, ["Cover - excluded by skip-cover", "Content - includes an interactive field", "Appendix - receives a different marker"][page - 1])
    if page == 2:
        c.acroForm.textfield(name="preserved_field", tooltip="Preserved interactive value", x=74, y=height-265, width=280, height=28, value="unchanged-value", fontName="Helvetica", fontSize=12)
    c.setFont("Helvetica", 10)
    c.drawRightString(width - 54, 38, f"Fixture page {page} of 3")
    c.showPage()
c.save()
