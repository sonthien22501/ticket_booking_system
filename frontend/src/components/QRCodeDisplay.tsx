import React, { useMemo } from 'react';

interface QRCodeDisplayProps {
  value: string;
  size?: number;
  className?: string;
  bgColor?: string;
  fgColor?: string;
}

/**
 * Deterministic SVG QR Matrix Generator
 * Generates standard QR-style 2D matrix with finder patterns, timing belts,
 * and deterministic hash-filled payload cells based on ticket payload.
 */
export const QRCodeDisplay: React.FC<QRCodeDisplayProps> = ({
  value,
  size = 160,
  className = '',
  bgColor = '#ffffff',
  fgColor = '#0F0F23',
}) => {
  const matrixSize = 25; // 25x25 QR Version 2 representation

  const matrix = useMemo(() => {
    // Initialize empty grid
    const grid: boolean[][] = Array.from({ length: matrixSize }, () =>
      Array(matrixSize).fill(false)
    );

    // Helper: draw finder pattern at (r, c)
    const drawFinder = (startRow: number, startCol: number) => {
      for (let r = 0; r < 7; r++) {
        for (let c = 0; c < 7; c++) {
          const isBorder = r === 0 || r === 6 || c === 0 || c === 6;
          const isCenter = r >= 2 && r <= 4 && c >= 2 && c <= 4;
          grid[startRow + r][startCol + c] = isBorder || isCenter;
        }
      }
    };

    // Draw 3 Finder Patterns
    drawFinder(0, 0); // Top-left
    drawFinder(0, matrixSize - 7); // Top-right
    drawFinder(matrixSize - 7, 0); // Bottom-left

    // Timing patterns
    for (let i = 8; i < matrixSize - 8; i++) {
      grid[6][i] = i % 2 === 0;
      grid[i][6] = i % 2 === 0;
    }

    // Alignment pattern (at 16, 16)
    const alignR = 16;
    const alignC = 16;
    for (let r = -2; r <= 2; r++) {
      for (let c = -2; c <= 2; c++) {
        const isBorder = Math.abs(r) === 2 || Math.abs(c) === 2;
        const isCenter = r === 0 && c === 0;
        grid[alignR + r][alignC + c] = isBorder || isCenter;
      }
    }

    // Seeded pseudo-random pseudo-hash based on value
    let hash = 0x811c9dc5;
    for (let i = 0; i < value.length; i++) {
      hash ^= value.charCodeAt(i);
      hash = Math.imul(hash, 0x01000193);
    }

    // Populate data cells (excluding finders, timing, alignment)
    for (let r = 0; r < matrixSize; r++) {
      for (let c = 0; c < matrixSize; c++) {
        // Skip finders and separators
        const inTopLeft = r <= 7 && c <= 7;
        const inTopRight = r <= 7 && c >= matrixSize - 8;
        const inBottomLeft = r >= matrixSize - 8 && c <= 7;
        const inAlign = Math.abs(r - alignR) <= 2 && Math.abs(c - alignC) <= 2;
        const inTiming = (r === 6 && c >= 8 && c < matrixSize - 8) || (c === 6 && r >= 8 && r < matrixSize - 8);

        if (!inTopLeft && !inTopRight && !inBottomLeft && !inAlign && !inTiming) {
          // LCG formula to generate deterministic pattern from payload hash
          hash = (hash * 1103515245 + 12345) & 0x7fffffff;
          grid[r][c] = (hash % 2) === 1;
        }
      }
    }

    return grid;
  }, [value]);

  const cellSize = size / matrixSize;

  return (
    <div
      className={`qr-code-wrapper inline-flex p-3 rounded-xl bg-white shadow-md border border-slate-200 ${className}`}
      style={{ width: size + 24, height: size + 24 }}
      role="img"
      aria-label={`QR Code for ${value}`}
    >
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        className="qr-code-svg block"
      >
        <rect width={size} height={size} fill={bgColor} />
        {matrix.map((row, r) =>
          row.map((cell, c) =>
            cell ? (
              <rect
                key={`${r}-${c}`}
                x={c * cellSize}
                y={r * cellSize}
                width={cellSize + 0.5}
                height={cellSize + 0.5}
                fill={fgColor}
              />
            ) : null
          )
        )}
      </svg>
    </div>
  );
};
