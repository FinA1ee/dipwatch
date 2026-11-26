import { useEffect, useRef, useState } from "react";

type Props = {
  value: string;
  options: readonly string[];
  onChange: (next: string) => void;
};

/**
 * The currency picker in the header: a button that opens a short menu under it.
 * A native select's popup is drawn by the OS and can open far from the page;
 * this one stays anchored. Arrow keys move between choices, Escape closes.
 */
export function CurrencyMenu({ value, options, onChange }: Props) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const toggle = useRef<HTMLButtonElement>(null);
  const items = useRef<(HTMLButtonElement | null)[]>([]);

  // Close on a click elsewhere; focus the current choice when the menu opens.
  useEffect(() => {
    if (!open) return;
    items.current[Math.max(options.indexOf(value), 0)]?.focus();
    const close = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [open, options, value]);

  const close = () => {
    setOpen(false);
    toggle.current?.focus();
  };
  const onItemKey = (e: React.KeyboardEvent, i: number) => {
    if (e.key === "Escape" || e.key === "Tab") {
      if (e.key === "Escape") e.preventDefault();
      close();
    } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const step = e.key === "ArrowDown" ? 1 : -1;
      items.current[(i + step + options.length) % options.length]?.focus();
    }
  };

  return (
    <div ref={root} className="relative">
      <button
        ref={toggle}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Currency: ${value}`}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setOpen(true);
          }
        }}
        className="flex items-center gap-1 rounded-lg border border-rule bg-white px-2.5 py-1 text-sm font-medium text-ink hover:border-muted"
      >
        {value}
        <svg aria-hidden="true" width="10" height="10" viewBox="0 0 10 10" className="text-muted">
          <path d="M1.5 3.5 5 7l3.5-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
        </svg>
      </button>
      {open && (
        <div
          role="menu"
          aria-label="Currency"
          className="absolute right-0 z-20 mt-1 min-w-[88px] rounded-lg border border-rule bg-white py-1 text-sm shadow-[0_6px_20px_rgba(24,34,46,0.12)]"
        >
          {options.map((c, i) => (
            <button
              key={c}
              ref={(el) => {
                items.current[i] = el;
              }}
              type="button"
              role="menuitemradio"
              aria-checked={c === value}
              onClick={() => {
                onChange(c);
                close();
              }}
              onKeyDown={(e) => onItemKey(e, i)}
              className={`flex w-full items-center justify-between gap-3 px-3 py-1.5 text-left hover:bg-paper focus-visible:bg-paper focus-visible:outline-none ${
                c === value ? "font-semibold text-drop" : "text-ink"
              }`}
            >
              {c}
              {c === value && <span aria-hidden="true">✓</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
