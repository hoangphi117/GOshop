import React from "react";
import {
  Cpu,
  HardDrive,
  Monitor,
  Battery,
  Camera,
  Layers,
  Volume2,
  Wifi,
  Sparkles,
  Zap,
} from "lucide-react";

interface SpecItem {
  icon: React.ComponentType<{ className?: string }>;
  value: string;
}

/**
 * Mapping of spec keys to suitable icon and clean value extraction.
 */
function getSpecItem(key: string, val: unknown): SpecItem | null {
  if (val === undefined || val === null) return null;
  const raw = String(val).trim();
  if (!raw) return null;

  const lowerKey = key.toLowerCase();

  // 1. CPU / Processor
  if (lowerKey.includes("cpu") || lowerKey.includes("processor") || lowerKey.includes("chip")) {
    return { icon: Cpu, value: raw };
  }

  // 2. RAM / Memory
  if (lowerKey === "ram" || lowerKey.includes("memory")) {
    return { icon: Zap, value: raw.replace(/\s*lpddr\w*/i, "") };
  }

  // 3. Storage / SSD / ROM / Hard Drive
  if (lowerKey.includes("storage") || lowerKey.includes("ssd") || lowerKey.includes("rom")) {
    return { icon: HardDrive, value: raw.replace(/\s*ufs\s*[\d.]+/i, "").replace(/\s*nvme/i, "") };
  }

  // 4. Display / Screen
  if (lowerKey.includes("display") || lowerKey.includes("screen")) {
    return { icon: Monitor, value: raw.replace(/\s*oled/i, "").replace(/\s*amoled/i, "").replace(/\s*120hz/i, "") };
  }

  // 5. Battery / Power
  if (lowerKey.includes("battery")) {
    return { icon: Battery, value: raw };
  }

  // 6. Camera / Sensor / Resolution
  if (lowerKey.includes("sensor") || lowerKey.includes("camera") || lowerKey.includes("resolution")) {
    return { icon: Camera, value: raw };
  }

  // 7. Audio / Sound / ANC
  if (lowerKey.includes("anc") || lowerKey.includes("audio") || lowerKey.includes("driver")) {
    return { icon: Volume2, value: raw.replace("Adaptive Active Noise Cancellation", "ANC") };
  }

  // 8. Connectivity / Wireless / Bluetooth
  if (lowerKey.includes("connect") || lowerKey.includes("bluetooth") || lowerKey.includes("wireless")) {
    return { icon: Wifi, value: raw };
  }

  // Fallback
  return { icon: Sparkles, value: raw };
}

interface ProductSpecBadgesProps {
  specifications?: Record<string, unknown> | null;
  maxItems?: number;
}

export function ProductSpecBadges({ specifications, maxItems = 4 }: ProductSpecBadgesProps) {
  if (!specifications || typeof specifications !== "object") return null;

  const entries = Object.entries(specifications);
  if (entries.length === 0) return null;

  // Prioritize meaningful keys: RAM, Storage, CPU, Battery, Display, Sensor, ANC
  const priorityOrder = [
    "ram",
    "storage",
    "cpu",
    "display",
    "battery",
    "battery life",
    "sensor resolution",
    "anc",
    "connectivity",
    "stabilization",
  ];

  const sortedEntries = [...entries].sort(([a], [b]) => {
    const idxA = priorityOrder.indexOf(a.toLowerCase());
    const idxB = priorityOrder.indexOf(b.toLowerCase());
    const rankA = idxA === -1 ? 999 : idxA;
    const rankB = idxB === -1 ? 999 : idxB;
    return rankA - rankB;
  });

  const items: SpecItem[] = [];
  for (const [k, v] of sortedEntries) {
    const item = getSpecItem(k, v);
    if (item) {
      items.push(item);
      if (items.length >= maxItems) break;
    }
  }

  if (items.length === 0) return null;

  return (
    <div className="bg-slate-50/90 rounded-md p-1.5 border border-slate-100 my-1.5 grid grid-cols-2 gap-x-2 gap-y-1">
      {items.map((item, idx) => {
        const IconComponent = item.icon;
        return (
          <div
            key={idx}
            className="flex items-center gap-1 min-w-0 text-[11px] font-medium text-slate-600"
            title={item.value}
          >
            <IconComponent className="w-3 h-3 text-slate-400 shrink-0" />
            <span className="truncate">{item.value}</span>
          </div>
        );
      })}
    </div>
  );
}
