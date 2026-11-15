// Unmodified test module excerpt from crates/pi-natives/src/snapcompact.rs
// Source: can1357/oh-my-pi@7c49c11aa5bbaf3f418406583096168ad5b8c6dc
// Copyright (c) 2025 Mario Zechner
// Copyright (c) 2025-2026 Can Bölük
// Copyright (c) 2026 Stencil Labs, Inc.
// SPDX-License-Identifier: MIT. Full license in adjacent LICENSE.
// Reference fixture only: Rust tests require their original upstream crate.

#[cfg(test)]
mod tests {
	use super::*;

	fn opts(size: u32) -> SnapcompactRenderOptions {
		SnapcompactRenderOptions { size, ..Default::default() }
	}

	#[test]
	fn fonts_parse_ascii_coverage() {
		for (font, ascent) in [(&*FONT_5X8, 7), (&*FONT_8X8, 7)] {
			assert_eq!(font.ascent, ascent);
			// Every printable ASCII char must have a glyph.
			for cp in 0x20u32..0x7f {
				assert!(font.glyphs.contains_key(&cp), "missing glyph for U+{cp:04X}");
			}
		}
	}

	#[test]
	fn silver_font_covers_cjk_scripts() {
		for ch in ['こ', '你', '안'] {
			assert_ne!(FONT_SILVER.face.lookup_glyph_index(ch), 0, "Silver must cover {ch:?}");
		}
	}

	#[test]
	fn digit_zero_is_disambiguated_from_letter_o() {
		// Regression for #8713: the default snapcompact bitmap fonts drew digit
		// `0` and letter `O` as bare ovals that OCR back ambiguously, corrupting
		// compacted identifiers. Each `0` now carries an interior slash/bar the
		// `O` lacks, so it inks strictly more of the glyph's vertical middle even
		// though it is the narrower oval (its wider top/bottom arcs sit outside
		// the sampled band). unscii-8 already shipped a slashed zero.
		for font in [&*FONT_5X8, &*FONT_6X12, &*FONT_8X13] {
			let (cw, ch) = (font.cell_w, font.cell_h);
			let width = cw * 2;
			let grid = Grid { cols: 2, rows: 1, repeat: 1, cell_w: cw, cell_h: ch };
			let px = render_bitmap("0O", width, ch, font, &grid, false, true);
			let band = ch / 4..ch - ch / 4;
			let mid_ink = |col0: usize| -> usize {
				band
					.clone()
					.flat_map(|y| (col0..col0 + cw).map(move |x| (x, y)))
					.filter(|&(x, y)| px[y * width + x] != 0)
					.count()
			};
			let (zero, oh) = (mid_ink(0), mid_ink(cw));
			assert!(
				zero > oh,
				"cell {cw}x{ch}: zero must ink its middle more than O (zero={zero}, O={oh})"
			);
		}
	}

	#[test]
	fn bitmap_inks_sentences_and_caps_capacity() {
		// 40px -> 8 cols x 5 rows = 40 cells (5x8 font).
		let grid = Grid { cols: 8, rows: 5, repeat: 1, cell_w: 5, cell_h: 8 };
		let pixels = render_bitmap("Hi. Ok.", 40, 40, &FONT_5X8, &grid, false, false);
		let inks: Vec<u8> = pixels.iter().copied().filter(|&p| p != 0).collect();
		assert!(inks.contains(&1), "first sentence should use ink 1");
		assert!(inks.contains(&2), "second sentence should use ink 2");
		assert!(!inks.contains(&3), "no third sentence ink expected");

		// Overflow input renders without panicking and stays in-bounds.
		let overflow = render_bitmap(&"x".repeat(100), 40, 40, &FONT_5X8, &grid, false, false);
		assert_eq!(overflow.len(), 40 * 40);
	}

	#[test]
	fn bw_variant_prints_black_only() {
		let grid = Grid { cols: 8, rows: 8, repeat: 1, cell_w: 8, cell_h: 8 };
		let pixels = render_bitmap("Hi. Ok.", 64, 64, &FONT_8X8, &grid, false, true);
		let inks: Vec<u8> = pixels.iter().copied().filter(|&p| p != 0).collect();
		assert!(!inks.is_empty());
		assert!(inks.iter().all(|&p| p == INK_BLACK), "bw must ink only black");
	}

	#[test]
	fn dim_markers_toggle_gray_without_consuming_cells() {
		let grid = Grid { cols: 8, rows: 8, repeat: 1, cell_w: 8, cell_h: 8 };
		let pixels = render_bitmap("\u{e}AB\u{f}CD", 64, 64, &FONT_8X8, &grid, false, true);
		let inks: Vec<u8> = pixels.iter().copied().filter(|&p| p != 0).collect();
		assert!(inks.contains(&INK_DIM), "dim span must ink gray");
		assert!(inks.contains(&INK_BLACK), "post-span text must return to black");
		// Markers are zero-width: glyphs land in the same cells as without them.
		let plain = render_bitmap("ABCD", 64, 64, &FONT_8X8, &grid, false, true);
		for (i, (a, b)) in pixels.iter().zip(&plain).enumerate() {
			assert_eq!(*a != 0, *b != 0, "cell layout must ignore markers (pixel {i})");
		}
	}

	#[test]
	fn line_repeat_duplicates_rows_on_highlight_bands() {
		// 64px, 8x8 font, repeat 2 -> 8 cols x 4 unique rows.
		let grid = Grid { cols: 8, rows: 4, repeat: 2, cell_w: 8, cell_h: 8 };
		let pixels = render_bitmap("ABCDEFGH", 64, 64, &FONT_8X8, &grid, false, true);
		// Copy band (rows 8..16) carries the highlight background.
		assert!(pixels[9 * 64..10 * 64].contains(&BG_REPEAT), "duplicate band must be highlighted");
		// Identical glyph ink in both copies: compare full 8-row bands modulo
		// background.
		for y in 0..8 {
			for x in 0..64 {
				let a = pixels[y * 64 + x];
				let b = pixels[(y + 8) * 64 + x];
				assert_eq!(a == INK_BLACK, b == INK_BLACK, "copy ink mismatch at ({x},{y})");
			}
		}
	}

	#[test]
	fn full_block_fills_cell_pitch_black() {
		let grid = Grid { cols: 8, rows: 4, repeat: 2, cell_w: 8, cell_h: 8 };
		// The block's black fill beats both the dim span and the sent hues.
		let pixels = render_bitmap("\u{e}a\u{2588}b\u{f}", 64, 64, &FONT_8X8, &grid, false, false);
		for copy in 0..2 {
			for y in copy * 8..(copy + 1) * 8 {
				for x in 8..16 {
					assert_eq!(pixels[y * 64 + x], INK_BLACK, "block pixel ({x},{y}) must be black");
				}
			}
		}
		assert!(pixels.contains(&INK_DIM), "neighbours keep their dim ink");
		let hued = render_bitmap("Hi.\u{2588}Ok.", 64, 64, &FONT_8X8, &grid, false, false);
		assert!(hued.contains(&2), "block must advance the sentence hue like a space");
	}

	#[test]
	fn doc_full_block_fills_cell() {
		// col_w = (13 - GUTTER) / 2 = 5; block at line 0, cell 1 -> x 8..16.
		let grid = Grid { cols: 13, rows: 2, repeat: 1, cell_w: 8, cell_h: 8 };
		let pixels = render_bitmap("a\u{2588}b\nc", 104, 16, &FONT_8X8, &grid, true, true);
		for y in 0..8 {
			for x in 8..16 {
				assert_eq!(pixels[y * 104 + x], INK_BLACK, "block pixel ({x},{y}) must be black");
			}
		}
	}

	/// Decode the base64 JS-string payload back to PNG bytes for inspection.
	fn png_bytes(encoded: Latin1String) -> Vec<u8> {
		STANDARD
			.decode(&*encoded)
			.expect("output must be valid base64")
	}

	#[test]
	fn render_native_is_indexed_and_stretch_is_rgb() {
		let native = png_bytes(
			render_snapcompact_png_sync("Hello world. Again.".into(), SnapcompactRenderOptions {
				size: 128,
				font: Some("8x8".into()),
				variant: Some("bw".into()),
				line_repeat: Some(2),
				..Default::default()
			})
			.unwrap(),
		);
		// PNG color type lives at byte 25 of the IHDR: 3 = indexed.
		assert_eq!(native[25], 3);

		let stretched = png_bytes(
			render_snapcompact_png_sync("Hello world. Again.".into(), SnapcompactRenderOptions {
				size: 128,
				font: Some("8x8".into()),
				cell_width: Some(6),
				cell_height: Some(6),
				..Default::default()
			})
			.unwrap(),
		);
		// 2 = truecolor RGB.
		assert_eq!(stretched[25], 2);
		let legacy = png_bytes(render_snapcompact_png_sync("Hi. Ok.".into(), opts(40)).unwrap());
		assert_eq!(legacy[25], 3, "default shape stays the legacy 5x8 indexed path");

		let silver = png_bytes(
			render_snapcompact_png_sync(
				"こんにちは 你好 안녕".into(),
				SnapcompactRenderOptions {
					size: 128,
					font: Some("silver".into()),
					cell_width: Some(16),
					cell_height: Some(16),
					variant: Some("bw".into()),
					..Default::default()
				},
			)
			.unwrap(),
		);
		assert_eq!(silver[25], 2, "TrueType frames render as RGB");
	}

	#[test]
	fn indexed_png_narrows_palette_and_bit_depth() {
		// IHDR bit depth lives at byte 24; PLTE length is the chunk length
		// word 8 bytes after the "PLTE" tag position.
		fn depth_and_palette(png: &[u8]) -> (u8, usize) {
			let tag = png
				.windows(4)
				.position(|w| w == b"PLTE")
				.expect("PLTE chunk");
			let len = u32::from_be_bytes(png[tag - 4..tag].try_into().unwrap()) as usize;
			(png[24], len / 3)
		}

		// Plain bw, no dim/band/repeat: background + black ink = 1-bit.
		let bw = png_bytes(
			render_snapcompact_png_sync("Hello world. Again.".into(), SnapcompactRenderOptions {
				size: 128,
				font: Some("8x8".into()),
				variant: Some("bw".into()),
				..Default::default()
			})
			.unwrap(),
		);
		assert_eq!(depth_and_palette(&bw), (1, 2));

		// bw with a dim span and repeat bands: 4 colors = 2-bit.
		let dim = png_bytes(
			render_snapcompact_png_sync(
				"Read \u{e}the dim part\u{f} now.".into(),
				SnapcompactRenderOptions {
					size: 128,
					font: Some("8x8".into()),
					variant: Some("bw".into()),
					line_repeat: Some(2),
					..Default::default()
				},
			)
			.unwrap(),
		);
		assert_eq!(depth_and_palette(&dim), (2, 4));

		// Sentence hues exceed 4 colors: stays 4-bit, palette still narrowed
		// to the inks actually printed (bg + 2 hues here).
		let sent = png_bytes(
			render_snapcompact_png_sync("Hi. Ok.".into(), SnapcompactRenderOptions {
				size: 128,
				font: Some("8x8".into()),
				variant: Some("sent".into()),
				..Default::default()
			})
			.unwrap(),
		);
		let (sent_depth, sent_colors) = depth_and_palette(&sent);
		assert_eq!(sent_depth, 2, "two hues + bg fit 2-bit");
		assert_eq!(sent_colors, 3);
	}

	#[test]
	fn rejects_bad_shapes() {
		assert!(render_snapcompact_png_sync("x".into(), opts(0)).is_err());
		assert!(
			render_snapcompact_png_sync("x".into(), SnapcompactRenderOptions {
				size: 64,
				font: Some("9x9".into()),
				..Default::default()
			})
			.is_err()
		);
		assert!(
			render_snapcompact_png_sync("x".into(), SnapcompactRenderOptions {
				size: 64,
				variant: Some("zebra".into()),
				..Default::default()
			})
			.is_err()
		);
	}

	#[test]
	fn xorg_fonts_parse_and_render() {
		for (font, ascent, name) in [(&*FONT_6X12, 10, "6x12"), (&*FONT_8X13, 11, "8x13")] {
			assert_eq!(font.ascent, ascent, "{name} ascent");
			for cp in 0x20u32..0x7f {
				assert!(font.glyphs.contains_key(&cp), "{name} missing glyph U+{cp:04X}");
			}
		}
		for (name, size) in [("6x12", 60u32), ("8x13", 104u32)] {
			let png = png_bytes(
				render_snapcompact_png_sync("Hello world. Again!".into(), SnapcompactRenderOptions {
					size,
					font: Some(name.into()),
					..Default::default()
				})
				.unwrap(),
			);
			assert_eq!(png[25], 3, "{name} natural cell must encode indexed");
		}
		// Non-blank: the raster must carry glyph ink.
		let grid = Grid { cols: 10, rows: 5, repeat: 1, cell_w: 6, cell_h: 12 };
		let pixels = render_bitmap("Hello", 60, 60, &FONT_6X12, &grid, false, true);
		assert!(pixels.contains(&INK_BLACK), "6x12 must ink pixels");
		let grid = Grid { cols: 8, rows: 8, repeat: 1, cell_w: 8, cell_h: 13 };
		let pixels = render_bitmap("Hello", 64, 104, &FONT_8X13, &grid, false, true);
		assert!(pixels.contains(&INK_BLACK), "8x13 must ink pixels");
	}

	#[test]
	fn stretch_false_renders_natural_glyphs_on_padded_pitch() {
		let png = png_bytes(
			render_snapcompact_png_sync(
				"Hello there. General Kenobi!".into(),
				SnapcompactRenderOptions {
					size: 128,
					font: Some("8x13".into()),
					cell_width: Some(8),
					cell_height: Some(16),
					stretch: Some(false),
					variant: Some("bw".into()),
					..Default::default()
				},
			)
			.unwrap(),
		);
		assert_eq!(png[25], 3, "8on16 must stay indexed");
		// IHDR width/height live at bytes 16..24, big-endian.
		let dim = |off: usize| u32::from_be_bytes(png[off..off + 4].try_into().unwrap());
		// "Hello there. General Kenobi!" is 28 chars on a 16-col grid: 2 rows
		// of the 16px pitch. 32px is under the 64px floor, so the frame is 64px
		// tall rather than padded to the 128px square.
		assert_eq!((dim(16), dim(20)), (128, 64), "declared geometry must match");

		// Glyph ink must sit in the top 13px of every 16px pitch row.
		let grid = Grid { cols: 16, rows: 8, repeat: 1, cell_w: 8, cell_h: 16 };
		let pixels =
			render_bitmap("Hgjpqy. Mixed descenders!", 128, 128, &FONT_8X13, &grid, false, true);
		assert!(pixels.contains(&INK_BLACK));
		for (i, &p) in pixels.iter().enumerate() {
			if p == INK_BLACK {
				assert!((i / 128) % 16 < 13, "ink leaked into pitch padding at y={}", i / 128);
			}
		}
	}

	#[test]
	fn doc_layout_flows_lines_into_second_column() {
		// 64px, 8x16 cells -> cols 8, rows 4, col_w = (8 - 3) / 2 = 2.
		let grid = Grid { cols: 8, rows: 4, repeat: 1, cell_w: 8, cell_h: 16 };
		let pixels = render_bitmap("A\nB\nC\nD\nE", 64, 64, &FONT_8X13, &grid, true, true);
		// Line 4 (the rows+1-th) lands at the second column's x origin:
		// 1 * (col_w + GUTTER) * cell_w = 40, row band 0.
		let col2 = (0..13).any(|y| (40..48).any(|x| pixels[y * 64 + x] == INK_BLACK));
		assert!(col2, "fifth line must start at the second column's x origin");
		// '\n' consumes no cell: line 1 starts back at x 0 in row band 1.
		let row1 = (16..29).any(|y| (0..8).any(|x| pixels[y * 64 + x] == INK_BLACK));
		assert!(row1, "second line must start at column 0 of the next row band");
		// One-char lines leave the rest of column 0 and the gutter blank.
		for y in 0..64 {
			for x in 8..40 {
				assert_eq!(pixels[y * 64 + x], 0, "gutter must stay blank at ({x},{y})");
			}
		}
	}

	#[test]
	fn doc_sentence_hue_advances_across_newline_boundary() {
		// 152px wide: cols 19, col_w = (19 - 3) / 2 = 8.
		let grid = Grid { cols: 19, rows: 4, repeat: 1, cell_w: 8, cell_h: 16 };
		let pixels = render_bitmap("Hi.\nOk", 152, 64, &FONT_8X13, &grid, true, false);
		let inks: Vec<u8> = pixels.iter().copied().filter(|&p| p != 0).collect();
		assert!(inks.contains(&1), "first sentence must use ink 1");
		assert!(inks.contains(&2), "hue must advance across the newline boundary");
		assert!(!inks.contains(&3), "no third sentence ink expected");

		// Grid mode keeps the space-only rule: no advance across '\n'.
		let gridmode = render_bitmap("Hi.\nOk", 152, 64, &FONT_8X13, &grid, false, false);
		let inks: Vec<u8> = gridmode.iter().copied().filter(|&p| p != 0).collect();
		assert!(inks.contains(&1));
		assert!(!inks.contains(&2), "grid mode must not advance hue across newline");
	}

	#[test]
	fn frame_height_hugs_used_rows_above_a_64px_floor() {
		let dims = |png: &[u8]| {
			let dim = |off: usize| u32::from_be_bytes(png[off..off + 4].try_into().unwrap());
			(dim(16), dim(20))
		};
		let render = |text: &str, opts: SnapcompactRenderOptions| {
			png_bytes(render_snapcompact_png_sync(text.into(), opts).unwrap())
		};
		let opts_8x8 =
			|| SnapcompactRenderOptions { size: 256, font: Some("8x8".into()), ..Default::default() };
		// 32 cols of 8x8 cells: 288 chars span 9 rows -> 72px tall.
		let nine_rows = "x".repeat(32 * 9);
		assert_eq!(dims(&render(&nine_rows, opts_8x8())), (256, 72));
		// Dim toggles are zero-width and must not add a row.
		assert_eq!(dims(&render(&format!("\u{e}{nine_rows}\u{f}"), opts_8x8())), (256, 72));
		// Capacity-filling text keeps the full grid height.
		assert_eq!(dims(&render(&"x".repeat(32 * 32), opts_8x8())), (256, 256));
		// One 8px row is padded to the 64px floor vision backends accept.
		assert_eq!(dims(&render("0123456789", opts_8x8())), (256, 64));
		// Repeat shapes hug `usedRows * repeat` copy bands.
		let repeated =
			render(&nine_rows, SnapcompactRenderOptions { line_repeat: Some(2), ..opts_8x8() });
		assert_eq!(dims(&repeated), (256, 144));
		// Doc layout counts `\n` lines down the first column: 5 rows of 16px.
		let doc = render("A\nB\nC\nD\nE", SnapcompactRenderOptions {
			size: 256,
			font: Some("8x13".into()),
			cell_width: Some(8),
			cell_height: Some(16),
			stretch: Some(false),
			columns: Some(2),
			..Default::default()
		});
		assert_eq!(dims(&doc), (256, 80));
		// The stretch path (RGB output, 6x6 target cells) hugs and floors too.
		let stretched = |text: &str| {
			render(text, SnapcompactRenderOptions {
				size: 120,
				font: Some("8x8".into()),
				cell_width: Some(6),
				cell_height: Some(6),
				..Default::default()
			})
		};
		assert_eq!(dims(&stretched(&"x".repeat(20 * 12))), (120, 72));
		assert_eq!(dims(&stretched("0123456789ab")), (120, 64));
	}

	#[test]
	fn columns_validates_and_renders_doc_frames() {
		assert!(
			render_snapcompact_png_sync("x".into(), SnapcompactRenderOptions {
				size: 64,
				columns: Some(3),
				..Default::default()
			})
			.is_err()
		);
		// Indexed doc frame (stretch: false on a padded pitch).
		let doc = png_bytes(
			render_snapcompact_png_sync(
				"Hello there.\nSecond line".into(),
				SnapcompactRenderOptions {
					size: 256,
					font: Some("8x13".into()),
					cell_width: Some(8),
					cell_height: Some(16),
					stretch: Some(false),
					columns: Some(2),
					..Default::default()
				},
			)
			.unwrap(),
		);
		assert_eq!(doc[25], 3, "8on16 doc frame must encode indexed");
		// Doc layout also applies on the stretch path (RGB output).
		let stretched = png_bytes(
			render_snapcompact_png_sync(
				"Hello there.\nSecond line".into(),
				SnapcompactRenderOptions {
					size: 256,
					font: Some("8x13".into()),
					cell_width: Some(6),
					cell_height: Some(12),
					columns: Some(2),
					..Default::default()
				},
			)
			.unwrap(),
		);
		assert_eq!(stretched[25], 2, "stretched doc frame must encode RGB");
	}
}
