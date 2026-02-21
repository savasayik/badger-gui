package ui

type layout struct {
	innerWidth         int
	innerHeight        int
	bodyHeight         int
	leftWidth          int
	rightWidth         int
	listWidth          int
	listHeight         int
	rightContentWidth  int
	rightContentHeight int
}

func computeLayout(width, height int) layout {
	innerWidth := width - (appPadX * 2)
	innerHeight := height - (appPadY * 2)
	if innerWidth < 10 {
		innerWidth = 10
	}
	if innerHeight < 6 {
		innerHeight = 6
	}

	bodyHeight := innerHeight - headerHeight - footerHeight
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	minLeft := 24
	minRight := 20
	leftWidth := int(float64(innerWidth) * 0.38)
	if innerWidth < minLeft+minRight+panelGap {
		leftWidth = innerWidth / 2
		if leftWidth < 10 {
			leftWidth = 10
		}
	} else {
		leftWidth = clamp(leftWidth, minLeft, innerWidth-minRight-panelGap)
	}
	rightWidth := innerWidth - leftWidth - panelGap
	if rightWidth < minRight {
		rightWidth = minRight
		leftWidth = innerWidth - rightWidth - panelGap
		if leftWidth < 10 {
			leftWidth = 10
		}
	}

	paneH := paneStyle.GetHorizontalFrameSize()
	paneV := paneStyle.GetVerticalFrameSize()

	listHeight := bodyHeight - paneV - panelHeaderLines
	if listHeight < 1 {
		listHeight = 1
	}
	listWidth := leftWidth - paneH
	if listWidth < 10 {
		listWidth = 10
	}

	rightContentHeight := bodyHeight - paneV - panelHeaderLines
	if rightContentHeight < 1 {
		rightContentHeight = 1
	}
	rightContentWidth := rightWidth - paneH
	if rightContentWidth < 10 {
		rightContentWidth = 10
	}

	return layout{
		innerWidth:         innerWidth,
		innerHeight:        innerHeight,
		bodyHeight:         bodyHeight,
		leftWidth:          leftWidth,
		rightWidth:         rightWidth,
		listWidth:          listWidth,
		listHeight:         listHeight,
		rightContentWidth:  rightContentWidth,
		rightContentHeight: rightContentHeight,
	}
}

func (m *Model) updateEditorLayout(lay layout) {
	m.editorHeight = lay.rightContentHeight
	if m.editorHeight < 1 {
		m.editorHeight = 1
	}
	m.editor.SetWidth(lay.rightContentWidth)
	m.editor.SetHeight(m.editorHeight)
}
