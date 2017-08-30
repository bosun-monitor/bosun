package rule

import (
	"bytes"
	"fmt"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule/parse"
	"github.com/pmezard/go-difflib/difflib"
)

// SaveRawText saves a new configuration file. The contextual diff of the change is provided
// to verify that no other changes have happened since the save request is issue. User, message, and
// args are passed to an optionally configured save hook. If the config file is not valid the file
// will not be saved. If the savehook fails to run or returns an error thaen the orginal config
// will be restored and the reload will not take place.
func (c *Conf) SaveRawText(rawConfig, diff, user, message string, args ...string) error {
	newConf, err := NewConf(c.Name, c.backends, c.sysVars, rawConfig)
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

// BulkEdit applies sequental edits to the configuration file. Each individual edit
// must generate a valid configuration or the edit request will fail.
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
		var l Location
		switch edit.Type {
		case "alert":
			a := newConf.GetAlert(edit.Name)
			if a != nil {
				l = a.Locator.(Location)
			}
		case "template":
			t := newConf.GetTemplate(edit.Name)
			if t != nil {
				l = t.Locator.(Location)
			}
		case "notification":
			n := newConf.GetNotification(edit.Name)
			if n != nil {
				l = n.Locator.(Location)
			}
		case "lookup":
			look := newConf.GetLookup(edit.Name)
			if look != nil {
				l = look.Locator.(Location)
			}
		case "macro":
			m := newConf.GetMacro(edit.Name)
			if m != nil {
				l = m.Locator.(Location)
			}
		default:
			return fmt.Errorf("%v is an unsuported type for bulk edit. must be alert, template, notification, lookup or macro", edit.Type)
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
		newConf, err = NewConf(c.Name, c.backends, c.sysVars, rawConf)
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

// Location stores the start byte position and end byte position of
// an object in the raw configuration
type Location []int

func writeSection(l Location, orginalRaw, newText string) string {
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

func removeSection(l Location, orginalRaw string) string {
	var newRawConf bytes.Buffer
	newRawConf.WriteString(orginalRaw[:getLocationStart(l)])
	newRawConf.WriteString(orginalRaw[getLocationEnd(l):])
	return newRawConf.String()
}

func newSectionLocator(s *parse.SectionNode) Location {
	start := int(s.Position())
	end := int(s.Position()) + len(s.RawText)
	return Location{start, end}
}

func getLocationStart(l Location) int {
	return l[0]
}

func getLocationEnd(l Location) int {
	return l[1]
}

// RawDiff returns a contextual diff of the running rule configuration
// against the provided configuration. This contextual diff library
// does not guarantee that the generated unified diff can be applied
// so this is only used for human consumption and verifying that the diff
// has not change since an edit request was issued
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
