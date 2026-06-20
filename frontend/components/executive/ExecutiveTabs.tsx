"use client";

// Hand-built accessible tabs (WAI-ARIA Tabs pattern) for the executive board
// report. Radix is NOT a dependency in this base-nova project, so this implements
// roving tabindex + Arrow/Home/End navigation directly. Inactive panels stay
// MOUNTED but `hidden` so the print stylesheet can reveal all sections at once.
import { useRef } from "react";
import type { LucideIcon } from "lucide-react";

export interface TabDef {
  key: string;
  label: string; // already translated by the caller
  icon: LucideIcon;
}

export function ExecutiveTabs({
  tabs,
  active,
  onChange,
  ariaLabel,
  panels,
}: {
  tabs: TabDef[];
  active: string;
  onChange: (key: string) => void;
  ariaLabel: string;
  panels: Record<string, React.ReactNode>;
}) {
  const btnRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  function focusTab(key: string) {
    onChange(key);
    // Move focus to the newly-activated trigger (roving tabindex).
    requestAnimationFrame(() => btnRefs.current[key]?.focus());
  }

  function onKeyDown(e: React.KeyboardEvent) {
    const i = tabs.findIndex((t) => t.key === active);
    if (i < 0) return;
    let next = -1;
    if (e.key === "ArrowRight") next = (i + 1) % tabs.length;
    else if (e.key === "ArrowLeft") next = (i - 1 + tabs.length) % tabs.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = tabs.length - 1;
    if (next >= 0) {
      e.preventDefault();
      focusTab(tabs[next].key);
    }
  }

  return (
    <div>
      <div
        role="tablist"
        aria-label={ariaLabel}
        onKeyDown={onKeyDown}
        className="flex gap-1 overflow-x-auto border-b border-hairline print:hidden"
      >
        {tabs.map((tab) => {
          const selected = tab.key === active;
          const Icon = tab.icon;
          return (
            <button
              key={tab.key}
              ref={(el) => {
                btnRefs.current[tab.key] = el;
              }}
              role="tab"
              id={`exec-tab-${tab.key}`}
              aria-selected={selected}
              aria-controls={`exec-panel-${tab.key}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => onChange(tab.key)}
              className={`-mb-px inline-flex shrink-0 items-center gap-2 whitespace-nowrap border-b-2 px-4 py-2.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 ${
                selected
                  ? "border-brand font-semibold text-foreground"
                  : "border-transparent font-medium text-muted-foreground hover:text-foreground"
              }`}
            >
              <Icon className="size-4" strokeWidth={1.75} aria-hidden />
              {tab.label}
            </button>
          );
        })}
      </div>

      {tabs.map((tab) => (
        <section
          key={tab.key}
          role="tabpanel"
          id={`exec-panel-${tab.key}`}
          aria-labelledby={`exec-tab-${tab.key}`}
          tabIndex={0}
          hidden={tab.key !== active}
          className="settle mt-6 focus-visible:outline-none"
        >
          {panels[tab.key]}
        </section>
      ))}
    </div>
  );
}
