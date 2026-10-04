package dialog

import (
	"time"

	"charm.land/lipgloss/v2"

	"github.com/oronbz/nag/internal/keybind"
	"github.com/oronbz/nag/internal/ui/components/datepicker"
	"github.com/oronbz/nag/internal/ui/components/selector"
	"github.com/oronbz/nag/internal/ui/styles"
)

// Local aliases keep the dialog package readable when composing the shared
// chooser and date picker.
type (
	selectorModel        = selector.Model
	selectorOption       = selector.Option
	selectorSelectedMsg  = selector.SelectedMsg
	selectorCancelledMsg = selector.CancelledMsg
	datepickerModel      = datepicker.Model
)

func newSelector(keys keybind.Map) selectorModel { return selector.New(keys) }

func datePickerNew(now, base time.Time, keys keybind.Map) datepickerModel {
	return datepicker.New(now, base, keys)
}

// dialogBoxStyle clamps the shared dialog chrome to the terminal width.
func dialogBoxStyle(availWidth int) lipgloss.Style {
	w := 74
	if availWidth > 0 && availWidth < w+4 {
		w = max(20, availWidth-4)
	}
	return styles.DialogStyle.Width(w)
}
