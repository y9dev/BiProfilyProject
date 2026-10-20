package widget

import (
	"os"
	"testing"

	"bigroup-backend/internal/model"
)

// TestWriteSample writes the evidence sample when WIDGET_SAMPLE_OUT is set.
func TestWriteSample(t *testing.T) {
	out := os.Getenv("WIDGET_SAMPLE_OUT")
	if out == "" {
		t.Skip("WIDGET_SAMPLE_OUT not set")
	}
	names := []string{"Коммуникация", "Лидерство", "Аналитика", "Командная работа", "Python", "SQL", "Дизайн"}
	vals := []float64{82, 64, 71, 55, 90, 40, 30}
	var ax []Axis
	for i, n := range names {
		ax = append(ax, Axis{Label: n, Value: vals[i], Core: i < 4})
	}
	h, err := Render(Data{Name: "Айгерим Сейтказиева", JobTitle: "Старший аналитик", Department: "Отдел продаж",
		Avatar: &model.Avatar{Universe: "anime", Gender: "female", Style: "tech"}, Axes: ax})
	if err != nil {
		t.Fatal(err)
	}
	page := `<!doctype html><meta charset="utf-8"><body style="margin:24px;background:#f4f6f9"><div style="width:320px">` + string(h) + `</div><div style="width:200px;margin-top:24px">` + string(h) + `</div></body>`
	if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
}
