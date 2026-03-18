package views

// Button Tailwind class constants.
const (
	btnBase      = "px-4 py-[7px] rounded-[7px] border-0 text-[13px] font-semibold cursor-pointer inline-flex items-center gap-1.5 tracking-[.01em] transition-[background,box-shadow] duration-150 active:scale-[.98] disabled:opacity-45 disabled:cursor-not-allowed"
	BtnPrimary   = btnBase + " bg-gradient-to-br from-[#7c6ef7] to-[#9a8eff] text-white shadow-[0_2px_10px_rgba(124,110,247,.25)] hover:shadow-[0_4px_16px_rgba(124,110,247,.25)]"
	BtnSecondary = btnBase + " bg-[#18182a] text-[#eeeeff] border border-white/[.12] hover:bg-[rgba(255,255,255,.07)]"
	BtnDanger    = btnBase + " bg-[rgba(248,113,113,.1)] text-[#f87171] border border-[rgba(248,113,113,.25)] hover:bg-[rgba(248,113,113,.2)]"

	// Badge Tailwind class constants.
	badgeBase      = "inline-flex items-center text-[10px] font-bold px-2 py-0.5 rounded-full whitespace-nowrap tracking-[.02em]"
	BadgeWarn      = badgeBase + " bg-[rgba(251,191,36,.12)] text-[#fbbf24] border border-[rgba(251,191,36,.25)]"
	BadgeOk        = badgeBase + " bg-[rgba(52,211,153,.12)] text-[#34d399] border border-[rgba(52,211,153,.25)]"
	BadgeDone      = badgeBase + " bg-[rgba(129,140,248,.12)] text-[#818cf8] border border-[rgba(129,140,248,.25)]"
	BadgeError     = badgeBase + " bg-[rgba(248,113,113,.12)] text-[#f87171] border border-[rgba(248,113,113,.25)]"
	BadgeBtn       = badgeBase + " cursor-pointer hover:opacity-80"
	BadgeDoneBtn   = BadgeDone + " cursor-pointer hover:bg-[rgba(248,113,113,.2)] hover:text-[#f87171] hover:border-[rgba(248,113,113,.4)]"
	BadgePlainBtn  = badgeBase + " cursor-pointer hover:opacity-80"
)

// ViewBtnClass returns the Tailwind classes for a view mode button.
// Active/inactive colours are handled by the .view-btn CSS rule in layout.templ
// so that syncViewButtons() only needs to toggle the "active" class.
func ViewBtnClass(active bool) string {
	base := "view-btn px-3 py-1 rounded-[4px] text-[12px] font-semibold no-underline transition-[background,color] duration-150 cursor-pointer"
	if active {
		return base + " active"
	}
	return base
}

// TypeTagClass returns the badge class for a media type tag.
func TypeTagClass(t string) string {
	base := "text-[9px] font-semibold px-1.5 py-px rounded-full"
	switch t {
	case "audio":
		return base + " bg-[rgba(56,189,248,.12)] text-[#38bdf8]"
	case "ebook":
		return base + " bg-[rgba(52,211,153,.12)] text-[#34d399]"
	default:
		return base + " bg-[rgba(255,255,255,.08)] text-[#7878a8]"
	}
}

// StatePillClass returns the class for a state pill.
func StatePillClass(active bool) string {
	base := "text-[9px] font-bold px-2 py-0.5 rounded-full border tracking-[.04em] transition-colors"
	if active {
		return base + " bg-[rgba(52,211,153,.18)] text-[#6ee7b7] border-[rgba(52,211,153,.4)]"
	}
	return base + " bg-[rgba(255,255,255,.04)] text-[#3a3a60] border-white/[.07]"
}
