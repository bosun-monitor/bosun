package rule

import (
	"bytes"
	"fmt"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule/parse"
	"github.com/pmezard/go-difflib/difflib"
)

func (c *Conf) SetAlert(name, alertText string) (string, error) {
	select {
	case c.writeLock <- true:
		// Got Write Lock
	default:
		return "", fmt.Errorf("cannot write alert, write in progress")
	}
	defer func() {
		<-c.writeLock
	}()
	a := c.GetAlert(name)
	var newRawConf string
	if a == nil {
		newRawConf = writeSection(nil, c.RawText, alertText)
	} else {
		newRawConf = writeSection(a.Locator, c.RawText, alertText)
	}
	newConf, err := NewConf(c.Name, c.backends, newRawConf)
	if err != nil {
		return "", fmt.Errorf("new config not valid: %v", err)
	}
	if err := c.SaveConf(newConf); err != nil {
		return "", fmt.Errorf("couldn't save config file: %v", err)
	}
	err = c.reload()
	if err != nil {
		return "", err
	}
	return "reloaded", nil
}

func (c *Conf) SaveRawText(rawConfig, diff, user, message string, args ...string) error {
	newConf, err := NewConf(c.Name, c.backends, rawConfig)
	if err != nil {
		return err
	}

	currentDiff, err := c.RawDiff(rawConfig)
	if err != nil {
		return fmt.Errorf("couldn't save config because failed to generate a diff: %v", err)
	}
	if currentDiff != diff {
		return fmt.Errorf("couldn't save config file because the change and supplied diff do not match the current diff")
	}
	if err = c.SaveConf(newConf); err != nil {
		return fmt.Errorf("couldn't save config file: %v", err)
	}
	if c.saveHook != nil {
		err := c.callSaveHook(c.Name, user, message, args...)
		if err != nil {
			sErr := c.SaveConf(c)
			restore := "successful"
			if sErr != nil {
				restore = sErr.Error()
			}
			return fmt.Errorf("failed to call save hook: %v. Restoring config: %v", err, restore)
		}
	}
	err = c.reload()
	if err != nil {
		return err
	}
	return nil
}

func (c *Conf) BulkEdit(edits conf.BulkEditRequest) error {
	select {
	case c.writeLock <- true:
		// Got Write Lock
	default:
		return fmt.Errorf("cannot write alert, write in progress")
	}
	defer func() {
		<-c.writeLock
	}()
	newConf := c
	var err error
	for _, edit := range edits {
		var l *conf.Locator
		switch edit.Type {
		case "alert":
			a := newConf.GetAlert(edit.Name)
			if a != nil {
				l = a.Locator
			}
		case "template":
			t := newConf.GetTemplate(edit.Name)
			if t != nil {
				l = t.Locator
			}
		case "notification":
			n := newConf.GetNotification(edit.Name)
			if n != nil {
				l = n.Locator
			}
		case "lookup":
			look := newConf.GetLookup(edit.Name)
			if look != nil {
				l = look.Locator
			}
		case "macro":
			m := newConf.GetMacro(edit.Name)
			if m != nil {
				l = m.Locator
			}
		default:
			return fmt.Errorf("%v is an unsuported type for bulk edit. must be alert, template, notification", edit.Type)
		}
		var rawConf string
		if edit.Delete {
			if l == nil {
				return fmt.Errorf("could not delete %v:%v - not found", edit.Type, edit.Name)
			}
			rawConf = removeSection(l, newConf.RawText)
		} else {
			rawConf = writeSection(l, newConf.RawText, edit.Text)
		}
		newConf, err = NewConf(c.Name, c.backends, rawConf)
		if err != nil {
			return fmt.Errorf("could not create new conf: failed on step %v:%v : %v", edit.Type, edit.Name, err)
		}
	}
	if err := c.SaveConf(newConf); err != nil {
		return fmt.Errorf("couldn't save config file: %v", err)
	}
	err = c.reload()
	if err != nil {
		return err
	}
	return nil
}

func (c *Conf) DeleteAlert(name string) error {
	select {
	case c.writeLock <- true:
		// Got Write Lock
	default:
		return fmt.Errorf("cannot delete alert, write in progress")
	}
	defer func() {
		<-c.writeLock
	}()
	a := c.GetAlert(name)
	if a == nil {
		return fmt.Errorf("alert %v not found", name)
	}
	newRawConf := removeSection(a.Locator, c.RawText)
	newConf, err := NewConf(c.Name, c.backends, newRawConf)
	if err != nil {
		return fmt.Errorf("new config not valid: %v", err)
	}
	if err := c.SaveConf(newConf); err != nil {
		return fmt.Errorf("couldn't save config file: %v", err)
	}
	err = c.reload()
	if err != nil {
		return err
	}
	return nil
}

func writeSection(l *conf.Locator, orginalRaw, newText string) string {
	var newRawConf bytes.Buffer
	if l == nil {
		newRawConf.WriteString(orginalRaw)
		newRawConf.WriteString("\n")
		newRawConf.WriteString(newText)
		newRawConf.WriteString("\n")
		return newRawConf.String()
	}
	newRawConf.WriteString(orginalRaw[:getLocationStart(l)])
	newRawConf.WriteString(newText)
	newRawConf.WriteString(orginalRaw[getLocationEnd(l):])
	return newRawConf.String()
}

func removeSection(l *conf.Locator, orginalRaw string) string {
	var newRawConf bytes.Buffer
	newRawConf.WriteString(orginalRaw[:getLocationStart(l)])
	newRawConf.WriteString(orginalRaw[getLocationEnd(l):])
	return newRawConf.String()
}

func newSectionLocator(s *parse.SectionNode) *conf.Locator {
	l := &conf.Locator{}
	start := int(s.Position())
	end := int(s.Position()) + len(s.RawText)
	l.Location = conf.NativeLocator{start, end}
	l.LocatorType = conf.TypeNative
	return l
}

func getLocationStart(l *conf.Locator) int {
	return l.Location.(conf.NativeLocator)[0]
}

func getLocationEnd(l *conf.Locator) int {
	return l.Location.(conf.NativeLocator)[1]
}

func (c *Conf) RawDiff(rawConf string) (string, error) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(c.RawText),
		B:        difflib.SplitLines(rawConf),
		FromFile: c.Name,
		ToFile:   c.Name,
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}
