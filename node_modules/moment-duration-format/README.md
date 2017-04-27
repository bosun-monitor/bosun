# Moment Duration Format

**Format plugin for the Moment Duration object.**

This is a plugin to the Moment.js JavaScript date library to add comprehensive formatting to Moment Durations.

Format template grammar is patterned on the existing Moment Date format template grammar, with a few modifications because durations are fundamentally different from dates.

This plugin does not have any dependencies beyond Moment.js itself, and may be used in the browser and in Node.js.

---

## Installation

**Node.js**

`npm install moment-duration-format`

**Bower**

`bower install moment-duration-format`

**Browser**

`<script src="path/to/moment-duration-format.js"></script>`

When using this plugin in the browser, be sure to include moment.js on your page first.

---

## Usage

### Module

To use this plugin as a module, use the `require` function:
```
require("moment-duration-format");
```

The plugin does not export anything, so there is no need to assign the require output to a variable.

The plugin depends on moment.js, which is not specified as a package dependency in the currently published version.

### Basics

The duration format method can format any moment duration. If no template or other arguments are provided, the default template function will generate a template string based on the duration's value.

```
moment.duration(123, "minutes").format();
// "2:03:00"

moment.duration(123, "months").format();
// "10y 3m"
```

The duration format method may be called with three optional arguments:
```
moment.duration.format([template] [, precision] [, settings])
```

### Template

`template` (string|function) is the string used to create the formatted output, or a function that returns the string to be used as the format template.

```
moment.duration(123, "minutes").format("h:mm");
// "2:03"
```

The template string is parsed for moment-token characters, which are replaced with the duration's value for each unit type. The default tokens are:
```
years:   Y or y
months:  M
weeks:   W or w
days:    D or d
hours:   H or h
minutes: m
seconds: s
ms:      S
```

Token characters may be customized ([see below for an example](\#tokens)).

Escape token characters within the template string using square brackets.
```
moment.duration(123, "minutes").format("h [hrs], m [min]");
// "2 hrs, 3 min"
```

Escape characters may also be customized ([see below for an example](\#escape)).



### Precision

`precision` (number) defines the number of digits to display for the final value.

The default precison value is `0`.
```
moment.duration(123, "minutes").format("h [hrs]");
// "2 hrs"
```

Positive precision defines the number of digits to display to the right of the decimal point.
```
moment.duration(123, "minutes").format("h [hrs]", 2);
// "2.04 hrs"
```

Negative precision will truncate the value to the left of the decimal point.
```
moment.duration(123, "minutes").format("m [min]", -1);
// "120 min"
```

### Settings

`settings` is an object that can override any of the default moment duration format options.

Both the `template` and `precision` arguments may be specified as properties of a single `settings` object argument, or they may be passed separately along with an optional settings object.

```
moment.duration(123, "minutes").format({ template: "h [hrs]", precision: 2 });
// "2.04 hrs"
```

#### Trim

Leading tokens are automatically trimmed when they have no value.
```
moment.duration(123, "minutes").format("d[d] h:mm:ss");
// "2:03:00"
```

To stop that behavior, set `{ trim: false }`.
```
moment.duration(123, "minutes").format("d[d] h:mm:ss", { trim: false });
// "0d 2:03:00"
```

Use `{ trim: "right" }` to trim from the right.
```
moment.duration(123, "minutes").format("[seconds:] s -- [minutes:] m -- [hours:] h -- [days:] d", { trim: "right" });
// "seconds: 0 -- minutes: 3 -- hours: 2"
```

#### Force Length

Force the first moment token with a value to render at full length, even when the template is trimmed and the first moment token has a length of 1. Sounds more complicated than it is.

```
moment.duration(123, "seconds").format("h:mm:ss");
// "2:03"
```

If you want minutes to always be rendered with two digits, you can set the first token to a length greater than 1 (this stops the automatic length trimming for the first token that has a value).

```
moment.duration(123, "seconds").format("hh:mm:ss");
// "02:03"
```

Or you can use `{ forceLength: true }`.

```
moment.duration(123, "seconds").format("h:mm:ss", { forceLength: true });
// "02:03"
```


### Tokens

Moment tokens are defined as regular expressions, and may be customized. Token definitions may be passed in the settings object, but are more likely set on the defaults object.

#### Escape

Default escape token regexp: `/\[(.+?)\]/`

Define something other than square brackets as escape characters.

```
moment.duration.fn.format.defaults.escape = /\((.+?)\)/;
moment.duration(123, "seconds").format("m (minutes)", 2);
// "2.04 minutes"
```

#### Years

Default years token regexp: `/[Yy]+/`

Define years token for spanish language formatting.

```
moment.duration.fn.format.defaults.years = /[Aa]+/;
moment.duration(123, "weeks").format("a [años]", 2);
// "2.35 años"
```

#### Months

Default months token regexp: `/M+/`

Define months token to use only lower-case `m`.

```
moment.duration.fn.format.defaults.months = /m+/;
moment.duration(123, "weeks").format("m M", 2);
// "28.36 M"
```

#### Weeks

Default weeks token regexp: `/[Ww]+/`

Define weeks token to use only lower-case `w`.

```
moment.duration.fn.format.defaults.weeks = /w+/;
moment.duration(123, "days").format("w W", 2);
// "17.57 W"
```

#### Days

Default days token regexp: `/[Dd]+/`

Define days token to use only lower-case `d`.

```
moment.duration.fn.format.defaults.days = /d+/;
moment.duration(123, "hours").format("d D", 2);
// "5.12 D"
```

#### Hours

Default hours token regexp: `/[Hh]+/`

Define hours token to use only lower-case `h`.

```
moment.duration.fn.format.defaults.hours = /h+/;
moment.duration(123, "minutes").format("h H", 2);
// "2.04 H"
```

#### Minutes

Default minutes token regexp: `/m+/`

Define minutes token to use only lower-case `n`.

```
moment.duration.fn.format.defaults.minutes = /n+/;
moment.duration(123, "seconds").format("n:ss");
// "2:03"
```

#### Seconds

Default seconds token regexp: `/s+/`

#### Milliseconds

Default milliseconds token regexp: `/S+/`

#### General

Default general token regexp: `/.+?/`

Not sure why you'd want to redefine the general token regexp, but you can. Just make sure it's lazy so the other token expressions can do their jobs.

