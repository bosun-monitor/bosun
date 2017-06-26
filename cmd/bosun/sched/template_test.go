package sched

var tstSimple = `
notification n {
	post = https://example.com/submit
	template = postData
}
notification e {
	email = test2@example.com
	template = body
}
template pduty {
	postData = '{"some":"json", "token":"{{.Alert.Vars.x}}","title":{{.Subject}}}'
}
template t {
	subject = "aaaaa"
	body = "some bad stuff happened"
	inherit pduty
}
alert a {
	$x = "foo"
	template = t
	crit = 1
    critNotification = n,e
}
`
