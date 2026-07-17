from pathlib import Path
import sys

from PIL import Image


SIZES = (16, 24, 32, 48, 64, 128, 256)


def main() -> int:
    root = Path(__file__).resolve().parent.parent
    source = root / "assets" / "cs2sj-logo.jpg"
    destination = root / "assets" / "cs2sj-logo.ico"
    if not source.is_file():
        print(f"Logo da HUD não encontrado: {source}", file=sys.stderr)
        return 1

    with Image.open(source) as image:
        image = image.convert("RGBA")
        width, height = image.size
        side = max(width, height)
        canvas = Image.new("RGBA", (side, side), (0, 0, 0, 255))
        canvas.alpha_composite(image, ((side - width) // 2, (side - height) // 2))
        canvas.save(destination, format="ICO", sizes=[(size, size) for size in SIZES])

    print(f"Ícone criado em {destination}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
