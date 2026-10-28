import { TicketTier } from '../types';

export interface Tilt3DResult {
  rotateX: number;
  rotateY: number;
  spotlightX: number;
  spotlightY: number;
  glareAngle: number;
}

export interface BoundingRectLike {
  left: number;
  top: number;
  width: number;
  height: number;
}

/**
 * Computes bounded 3D perspective rotation angles and radial spotlight coordinates
 * from mouse cursor coordinates relative to a card or boarding pass element.
 */
export function calculate3DTilt(
  rect: BoundingRectLike,
  clientX: number,
  clientY: number,
  maxDeg: number = 7
): Tilt3DResult {
  if (!rect || rect.width <= 0 || rect.height <= 0) {
    return {
      rotateX: 0,
      rotateY: 0,
      spotlightX: 50,
      spotlightY: 50,
      glareAngle: 135,
    };
  }

  const relX = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
  const relY = Math.max(0, Math.min(1, (clientY - rect.top) / rect.height));

  // Center is (0.5, 0.5)
  const normX = (relX - 0.5) * 2; // -1 to 1
  const normY = (relY - 0.5) * 2; // -1 to 1

  // Tilting towards top-left means positive rotateX and negative rotateY
  const rotateX = Number((-normY * maxDeg).toFixed(2));
  const rotateY = Number((normX * maxDeg).toFixed(2));
  const spotlightX = Number((relX * 100).toFixed(1));
  const spotlightY = Number((relY * 100).toFixed(1));
  const glareAngle = Number((Math.atan2(normY, normX) * (180 / Math.PI) + 135).toFixed(1));

  return {
    rotateX: rotateX === 0 ? 0 : rotateX,
    rotateY: rotateY === 0 ? 0 : rotateY,
    spotlightX,
    spotlightY,
    glareAngle,
  };
}

/**
 * Cubic ease-out interpolation for smooth KPI number count-up animations.
 */
export function interpolateCountUp(start: number, end: number, progress: number): number {
  const clamped = Math.max(0, Math.min(1, progress));
  const eased = 1 - Math.pow(1 - clamped, 3);
  return start + (end - start) * eased;
}

/**
 * Calculates staggered entrance animation delay (in ms) with a safe upper cap
 * so large grids or 100-seat matrices remain responsive.
 */
export function calculateStaggerDelay(
  index: number,
  baseMs: number = 55,
  maxMs: number = 600
): number {
  if (index <= 0) return 0;
  return Math.min(index * baseMs, maxMs);
}

/**
 * Computes SVG strokeDasharray (circumference) and strokeDashoffset for
 * circular occupancy gauges.
 */
export function calculateRingDashOffset(
  percent: number,
  radius: number = 20
): { circumference: number; dashOffset: number } {
  const clamped = Math.max(0, Math.min(100, percent));
  const circumference = 2 * Math.PI * radius;
  const dashOffset = circumference * (1 - clamped / 100);
  return { circumference, dashOffset };
}

/**
 * Computes real-time seat scarcity metrics across ticket tiers for visual urgency bars.
 */
export function calculateSeatScarcity(tiers: TicketTier[]): {
  totalAvailable: number;
  totalCapacity: number;
  soldPercent: number;
  isHighDemand: boolean;
} {
  if (!tiers || tiers.length === 0) {
    return { totalAvailable: 0, totalCapacity: 0, soldPercent: 0, isHighDemand: false };
  }

  let totalAvailable = 0;
  let totalCapacity = 0;

  for (const tier of tiers) {
    const avail = tier.availableSeats ?? 0;
    const cap = tier.capacity && tier.capacity > 0 ? tier.capacity : Math.max(avail, 40);
    totalAvailable += avail;
    totalCapacity += cap;
  }

  if (totalCapacity <= 0) {
    return { totalAvailable, totalCapacity: 0, soldPercent: 0, isHighDemand: false };
  }

  const soldRatio = Math.max(0, Math.min(1, (totalCapacity - totalAvailable) / totalCapacity));
  const soldPercent = Math.round(soldRatio * 100);
  const isHighDemand = soldPercent >= 65 || (totalAvailable > 0 && totalAvailable <= 20);

  return {
    totalAvailable,
    totalCapacity,
    soldPercent,
    isHighDemand,
  };
}
