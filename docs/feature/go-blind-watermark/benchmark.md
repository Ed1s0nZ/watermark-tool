# Go DCT v1 real-image benchmark

UTC: 2026-10-26T10:28:18Z

Public benchmark key only; text: 项目-A100. Negative rows: true means correctly rejected. PSNR compares decoded output against original decoded RGB. 100 means identical (sentinel).


## flower.jpg

| Image | Size | Case | Pass | PSNR dB | ms | Error |
|---|---|---|---|---|---|---|
| flower.jpg | 500x477 | png-reencode | true | 33.46 | 56 |  |
| flower.jpg | 500x477 | jpeg-95 | true | 33.37 | 39 |  |
| flower.jpg | 500x477 | jpeg-85 | true | 32.61 | 39 |  |
| flower.jpg | 500x477 | jpeg-75 | true | 31.76 | 44 |  |
| flower.jpg | 500x477 | clean-negative | true | N/A | 4 |  |
| flower.jpg | 500x477 | wrong-key-negative | true | N/A | 0 |  |

Source: https://upload.wikimedia.org/wikipedia/commons/3/3f/JPEG_example_flower.jpg

SHA256: `c160fa1bc488af7f48984bd627bc5cbc5a20b98eaaf11af92e99a206b376252d`; original intact: true


## astronaut.png

| Image | Size | Case | Pass | PSNR dB | ms | Error |
|---|---|---|---|---|---|---|
| astronaut.png | 512x512 | png-reencode | true | 31.90 | 68 |  |
| astronaut.png | 512x512 | jpeg-95 | true | 30.82 | 52 |  |
| astronaut.png | 512x512 | jpeg-85 | true | 30.14 | 51 |  |
| astronaut.png | 512x512 | jpeg-75 | true | 29.68 | 50 |  |
| astronaut.png | 512x512 | clean-negative | true | N/A | 3 |  |
| astronaut.png | 512x512 | wrong-key-negative | true | N/A | 0 |  |

Source: https://raw.githubusercontent.com/scikit-image/scikit-image/v0.19.3/skimage/data/astronaut.png

SHA256: `88431cd9653ccd539741b555fb0a46b61558b301d4110412b5bc28b5e3ea6cb5`; original intact: true


## coffee.png

| Image | Size | Case | Pass | PSNR dB | ms | Error |
|---|---|---|---|---|---|---|
| coffee.png | 600x400 | png-reencode | true | 32.23 | 66 |  |
| coffee.png | 600x400 | jpeg-95 | true | 30.91 | 51 |  |
| coffee.png | 600x400 | jpeg-85 | true | 29.92 | 51 |  |
| coffee.png | 600x400 | jpeg-75 | true | 29.19 | 50 |  |
| coffee.png | 600x400 | clean-negative | true | N/A | 2 |  |
| coffee.png | 600x400 | wrong-key-negative | true | N/A | 0 |  |

Source: https://raw.githubusercontent.com/scikit-image/scikit-image/v0.19.3/skimage/data/coffee.png

SHA256: `cc02f8ca188b167c775a7101b5d767d1e71792cf762c33d6fa15a4599b5a8de7`; original intact: true

