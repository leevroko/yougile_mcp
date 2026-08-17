package valueobject

// PagingMetadata — пагинация (совпадает с API YouGile).
type PagingMetadata struct {
	Count  int  // количество элементов в ответе
	Limit  int  // максимум на страницу
	Offset int  // индекс первого элемента
	Next   bool // есть ли ещё страницы
}

// HasNext возвращает true, если есть ещё страницы.
func (p PagingMetadata) HasNext() bool { return p.Next }

// ZeroPaging — пустая пагинация (для тестов/пустых результатов).
func ZeroPaging() PagingMetadata { return PagingMetadata{} }
