package conf

import (
	"bosun.org/cmd/bosun/conf/parse"
)

func (c *Conf) replacableItems() map[string]*ReplacableItem {
	list := map[string]*ReplacableItem{}
	for k, v := range c.Alerts {
		list["a"+k] = &v.ReplacableItem
	}
	for k, v := range c.Macros {
		list["m"+k] = &v.ReplacableItem
	}
	for k, v := range c.Lookups {
		list["l"+k] = &v.ReplacableItem
	}
	for k, v := range c.Notifications {
		list["n"+k] = &v.ReplacableItem
	}
	for k, v := range c.Templates {
		list["t"+k] = &v.ReplacableItem
	}
	return list
}

func (c *Conf) Merge(other *Conf) *Conf {
	c, _ = New(c.Name, c.RawText)
	existing := c.replacableItems()
	for name, i := range other.replacableItems() {
		old, ok := existing[name]
		if !ok {
			c.RawText += "\n" + i.Def + "\n"
		} else {
			c.replaceRawText(old, i.Def)
		}
	}
	c, _ = New(c.Name, c.RawText)
	return c
}

func (c *Conf) replaceRawText(r *ReplacableItem, text string) {
	before := c.RawText[0:r.Pos]
	after := c.RawText[int(r.Pos)+len(r.Def):]
	c.RawText = before + text + after
	diff := len(text) - len(r.Def)
	r.Def = text

	for _, item := range c.replacableItems() {
		if item.Pos > r.Pos {
			item.Pos += parse.Pos(diff)
		}
	}
}
