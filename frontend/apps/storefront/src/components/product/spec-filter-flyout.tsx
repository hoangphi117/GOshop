"use client";

import React, { useState, useEffect } from "react";
import { useRouter, useSearchParams, usePathname } from "next/navigation";
import { ChevronRight, Check, X, Cpu, RotateCcw } from "lucide-react";
import type { FacetGroup } from "@/lib/api-server";

interface SpecFilterFlyoutProps {
  facets: FacetGroup[];
  currentSpecs: Record<string, string>;
}

export function SpecFilterFlyout({ facets, currentSpecs }: SpecFilterFlyoutProps) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  // Track pending selection for each facet key: { "RAM": ["16GB", "32GB"] }
  const [pendingSelections, setPendingSelections] = useState<Record<string, string[]>>({});

  // Sync state whenever URL currentSpecs changes
  useEffect(() => {
    const initial: Record<string, string[]> = {};
    Object.entries(currentSpecs).forEach(([key, val]) => {
      if (val) {
        initial[key] = val
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean);
      }
    });
    setPendingSelections(initial);
  }, [currentSpecs]);

  const getFacetSelected = (facetKey: string): string[] => {
    if (pendingSelections[facetKey] !== undefined) {
      return pendingSelections[facetKey];
    }
    const val = currentSpecs[facetKey];
    if (!val) return [];
    return val
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  };

  const toggleOption = (facetKey: string, optionValue: string) => {
    const current = getFacetSelected(facetKey);
    const exists = current.includes(optionValue);
    const updated = exists
      ? current.filter((v) => v !== optionValue)
      : [...current, optionValue];

    setPendingSelections((prev) => ({
      ...prev,
      [facetKey]: updated,
    }));
  };

  const handleApply = (facetKey: string) => {
    const selected = getFacetSelected(facetKey);
    const params = new URLSearchParams(searchParams.toString());
    params.set("page", "1");

    if (selected.length > 0) {
      params.set(`spec_${facetKey}`, selected.join(","));
    } else {
      params.delete(`spec_${facetKey}`);
    }

    const qs = params.toString();
    router.push(`${pathname}${qs ? `?${qs}` : ""}`);
  };

  const handleClear = (facetKey: string) => {
    setPendingSelections((prev) => ({
      ...prev,
      [facetKey]: [],
    }));

    const params = new URLSearchParams(searchParams.toString());
    params.set("page", "1");
    params.delete(`spec_${facetKey}`);

    const qs = params.toString();
    router.push(`${pathname}${qs ? `?${qs}` : ""}`);
  };

  const handleClearAll = () => {
    setPendingSelections({});
    const params = new URLSearchParams(searchParams.toString());
    params.set("page", "1");

    Array.from(params.keys()).forEach((key) => {
      if (key.startsWith("spec_")) {
        params.delete(key);
      }
    });

    const qs = params.toString();
    router.push(`${pathname}${qs ? `?${qs}` : ""}`);
  };

  const activeSpecKeys = Object.keys(currentSpecs).filter(
    (k) => currentSpecs[k] && currentSpecs[k].trim() !== ""
  );

  const validFacets = facets.filter((f) => f.options && f.options.length > 0);
  if (validFacets.length === 0) {
    return null;
  }

  return (
    <div className="bg-white border border-slate-200/90 rounded-xs p-4 shadow-xs relative z-20">
      {/* Header */}
      <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-3">
        <div className="flex items-center gap-2 text-xs font-bold text-slate-900 uppercase tracking-wider font-heading">
          <Cpu className="w-3.5 h-3.5 text-[#007D9C]" />
          <span>Specifications</span>
        </div>
        {activeSpecKeys.length > 0 && (
          <button
            type="button"
            onClick={handleClearAll}
            className="text-[11px] font-semibold text-[#007D9C] hover:underline cursor-pointer"
          >
            Clear All
          </button>
        )}
      </div>

      {/* Specifications list with hover flyout */}
      <div className="space-y-1">
        {validFacets.map((facet) => {
          const appliedRaw = currentSpecs[facet.key];
          const appliedList = appliedRaw
            ? appliedRaw
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean)
            : [];
          const isApplied = appliedList.length > 0;
          const pendingList = getFacetSelected(facet.key);

          return (
            <div key={facet.key} className="relative group group-hover:z-30">
              {/* Trigger row */}
              <div
                className={`flex items-center justify-between px-3 py-2 rounded-xs text-xs font-semibold cursor-pointer transition-colors ${
                  isApplied
                    ? "bg-[#E0F7FC] text-[#007D9C] font-bold font-heading"
                    : "text-slate-700 hover:bg-[#F0F9FB] hover:text-[#007D9C]"
                }`}
              >
                <div className="flex items-center gap-2 min-w-0 pr-2">
                  <span className="truncate">{facet.key}</span>
                  {isApplied && (
                    <span className="text-[10px] bg-[#007D9C] text-white px-1.5 py-0.2 rounded-xs truncate max-w-[85px]">
                      {appliedList.length === 1
                        ? appliedList[0]
                        : `${appliedList.length} selected`}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  {isApplied && (
                    <button
                      type="button"
                      title={`Clear ${facet.key}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        handleClear(facet.key);
                      }}
                      className="text-slate-400 hover:text-red-600 p-0.5 rounded-xs"
                    >
                      <X className="w-3 h-3" />
                    </button>
                  )}
                  <ChevronRight className="w-3.5 h-3.5 text-slate-400 group-hover:text-[#007D9C] group-hover:translate-x-0.5 transition-transform" />
                </div>
              </div>

              {/* Flyout Menu Beside (strictly rounded-md) */}
              <div className="invisible opacity-0 pointer-events-none group-hover:visible group-hover:opacity-100 group-hover:pointer-events-auto transition-all duration-150 ease-out absolute z-50 left-0 top-full mt-1 lg:left-full lg:top-0 lg:mt-0 lg:ml-1.5 w-68 sm:w-76 bg-white border border-slate-200/90 shadow-xl rounded-md p-3.5 before:hidden lg:before:block before:absolute before:-left-2 before:top-0 before:w-2 before:h-full before:content-['']">
                {/* Flyout Top Info */}
                <div className="flex items-center justify-between pb-2 mb-2 border-b border-slate-100">
                  <div className="flex items-center gap-1.5 text-xs font-bold text-slate-900 font-heading">
                    <span>{facet.key}</span>
                    <span className="text-[10px] text-slate-400 font-normal">
                      ({facet.options.length} options)
                    </span>
                  </div>
                  {pendingList.length > 0 && (
                    <span className="text-[10px] font-bold text-[#007D9C] bg-[#E0F7FC] px-1.5 py-0.2 rounded-xs">
                      {pendingList.length} selected
                    </span>
                  )}
                </div>

                {/* Scrollable Checkbox Options */}
                <div className="space-y-1 max-h-60 overflow-y-auto custom-scrollbar pr-0.5">
                  {facet.options.map((opt) => {
                    const isChecked = pendingList.includes(opt.value);
                    return (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() => toggleOption(facet.key, opt.value)}
                        className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-xs text-xs text-left transition-colors cursor-pointer ${
                          isChecked
                            ? "bg-[#E0F7FC] text-[#007D9C] font-bold font-heading"
                            : "text-slate-700 hover:bg-slate-50 font-body"
                        }`}
                      >
                        <div className="flex items-center gap-2 truncate pr-2">
                          <div
                            className={`w-3.5 h-3.5 rounded-xs border flex items-center justify-center shrink-0 transition-colors ${
                              isChecked
                                ? "bg-[#007D9C] border-[#007D9C]"
                                : "border-slate-300 bg-white"
                            }`}
                          >
                            {isChecked && <Check className="w-2.5 h-2.5 text-white stroke-[3]" />}
                          </div>
                          <span className="truncate">{opt.value}</span>
                        </div>
                        <span
                          className={`text-[10px] px-1.5 py-0.2 rounded-full shrink-0 font-medium ${
                            isChecked
                              ? "bg-[#007D9C] text-white"
                              : "bg-slate-100 text-slate-500"
                          }`}
                        >
                          {opt.count}
                        </span>
                      </button>
                    );
                  })}
                </div>

                {/* Flyout Actions: Clear (left) and Apply (right) with equal width */}
                <div className="pt-2.5 mt-2.5 border-t border-slate-100 flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => handleClear(facet.key)}
                    className="flex-1 py-1.5 px-3 border border-slate-200 hover:bg-slate-100 text-slate-600 text-xs font-semibold rounded-xs transition-colors cursor-pointer flex items-center justify-center gap-1 shadow-xs"
                  >
                    <RotateCcw className="w-3.5 h-3.5 text-slate-400" />
                    <span>Clear</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => handleApply(facet.key)}
                    className="flex-1 py-1.5 px-3 bg-[#007D9C] hover:bg-[#00637C] text-white text-xs font-bold rounded-xs flex items-center justify-center gap-1 font-heading transition-colors cursor-pointer shadow-xs"
                  >
                    <Check className="w-3.5 h-3.5" />
                    <span>Apply{pendingList.length > 0 ? ` (${pendingList.length})` : ""}</span>
                  </button>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
