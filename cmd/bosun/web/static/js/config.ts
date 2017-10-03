/// <reference path="0-bosun.ts" />
interface IConfigScope extends IBosunScope {
	// text loading/navigation
	config_text: string;
	selected_alert: string;
	items: { [type: string]: string[]; };
	scrollTo: (type: string, name: string) => void;
	aceLoaded: (editor: any) => void;
	editor: any;
	validate: () => void;
	validationResult: string;
	saveResult: string;
	selectAlert: (alert: string) => void;
	reparse: () => void;
	aceTheme: string;
	aceMode: string;
	aceToggleHighlight: () => void;
	quickJumpTarget: string;
	quickJump: () => void;
	downloadConfig: () => void;
	saveConfig: () => void;
	saveClass: () => string;
	sectionToDocs: { [type: string]: string; };

	//rule execution options
	fromDate: string;
	toDate: string;
	fromTime: string;
	toTime: string;
	intervals: number;
	duration: number;
	email: string;
	template_group: string;
	setInterval: () => void;
	setDuration: () => void;

	//rule execution
	running: boolean;
	error: string;
	warning: string[];
	test: () => void;
	sets: any;
	alert_history: any;
	subject: string;
	emailSubject: string;
	emailBody: string;
	body: string;
	customTemplates: { [name: string]: string };
	notifications: { [name: string]: any };
	actionNotifications: {[name: string]: {[at: string]:any}};
	notificationToShow: string;
	data: any;
	tab: string;
	zws: (v: string) => string;
	setTemplateGroup: (group: any) => void;
	scrollToInterval: (v: string) => void;
	show: (v: any) => void;
	loadTimelinePanel: (entry: any, v: any) => void;
	incidentId: number;

	// saving
	message: string;
	diff: string;
	diffConfig: () => void;
	expandDiff: boolean;
	runningHash: string;
	runningChanged: boolean;
	runningChangedHelp: string;
	runningHashResult: string;
	getRunningHash: () => void;
}

bosunControllers.controller('ConfigCtrl', ['$scope', '$http', '$location', '$route', '$timeout', '$sce', function ($scope: IConfigScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService, $timeout: ng.ITimeoutService, $sce: ng.ISCEService) {
	var search = $location.search();
	$scope.fromDate = search.fromDate || '';
	$scope.fromTime = search.fromTime || '';
	$scope.toDate = search.toDate || '';
	$scope.toTime = search.toTime || '';
	$scope.intervals = +search.intervals || 5;
	$scope.duration = +search.duration || null;
	$scope.runningHash = search.runningHash || null;
	$scope.runningChanged = search.runningChanged || false;
	$scope.config_text = 'Loading config...';
	$scope.selected_alert = search.alert || '';
	$scope.email = search.email || '';
	$scope.template_group = search.template_group || '';
	$scope.items = parseItems();
	$scope.tab = search.tab || 'results';
	$scope.aceTheme = 'chrome';
	$scope.actionTypeToShow = "Acknowledged";
	$scope.incidentId = 42;

	$scope.aceMode = 'bosun';
	$scope.expandDiff = false;
	$scope.customTemplates = {};
	$scope.runningChangedHelp = "The running config has been changed. This means you are in danger of overwriting someone else's changes. To view the changes open the 'Save Dialogue' and you will see a unified diff. The only way to get rid of the error panel is to open a new instance of the rule editor and copy your changes into it. You are still permitted to save without doing this, but then you must be very careful not to overwrite anyone else's changes.";

	$scope.sectionToDocs = {
		"alert": "https://bosun.org/definitions#alert-definitions",
		"template": "https://bosun.org/definitions#templates",
		"lookup": "https://bosun.org/definitions#lookup-tables",
		"notification": "https://bosun.org/definitions#notifications",
		"macro": "https://bosun.org/definitions#macros"
	}

	var expr = search.expr;
	function buildAlertFromExpr() {
		if (!expr) return;
		var newAlertName = "test";
		var idx = 1;
		//find a unique alert name
		while ($scope.items["alert"].indexOf(newAlertName) != -1 || $scope.items["template"].indexOf(newAlertName) != -1) {
			newAlertName = "test" + idx;
			idx++;
		}
		var text = '\n\ntemplate ' + newAlertName + ' {\n' +
			'	subject = {{.Last.Status}}: {{.Alert.Name}} on {{.Group.host}}\n' +
			'	body = `<p>Name: {{.Alert.Name}}\n' +
			'	<p>Tags:\n' +
			'	<table>\n' +
			'		{{range $k, $v := .Group}}\n' +
			'			<tr><td>{{$k}}</td><td>{{$v}}</td></tr>\n' +
			'		{{end}}\n' +
			'	</table>`\n' +
			'}\n\n';
		var expression = atob(expr);
		var lines = expression.split("\n").map(function (l) { return l.trim(); });
		lines[lines.length - 1] = "crit = " + lines[lines.length - 1]
		expression = lines.join("\n    ");
		text += 'alert ' + newAlertName + ' {\n' +
			'	template = ' + newAlertName + '\n' +
			'	' + expression + '\n' +
			'}\n';
		$scope.config_text += text;
		$scope.items = parseItems();
		$timeout(() => {
			//can't scroll editor until after control is updated. Defer it.
			$scope.scrollTo("alert", newAlertName);
		})
	}

	function parseItems(): { [type: string]: string[]; } {
		var configText = $scope.config_text;
		var re = /^\s*(alert|template|notification|lookup|macro)\s+([\w\-\.\$]+)\s*\{/gm;
		var match;
		var items: { [type: string]: string[]; } = {};
		items["alert"] = [];
		items["template"] = [];
		items["lookup"] = [];
		items["notification"] = [];
		items["macro"] = [];
		while (match = re.exec(configText)) {
			var type = match[1];
			var name = match[2];
			var list = items[type];
			if (!list) {
				list = [];
				items[type] = list;
			}
			list.push(name);
		}
		return items;
	}

	$http.get('/api/config?hash=' + (search.hash || ''))
		.success((data: any) => {
			$scope.config_text = data;
			$scope.items = parseItems();
			buildAlertFromExpr();
			if (!$scope.selected_alert && $scope.items["alert"].length) {
				$scope.selected_alert = $scope.items["alert"][0];
			}
			$timeout(() => {
				//can't scroll editor until after control is updated. Defer it.
				$scope.scrollTo("alert", $scope.selected_alert);
			})

		})
		.error(function (data) {
			$scope.validationResult = "Error fetching config: " + data;
		})

	$scope.reparse = function () {
		$scope.items = parseItems();
	}
	var editor;
	$scope.aceLoaded = function (_editor) {
		editor = _editor;
		$scope.editor = editor;
		editor.focus();
		editor.getSession().setUseWrapMode(true);
		editor.on("blur", function () {
			$scope.$apply(function () {
				$scope.items = parseItems();
			});
		});
	};
	var syntax = true;
	$scope.aceToggleHighlight = function () {
		if (syntax) {
			editor.getSession().setMode();
			syntax = false;
			return;
		}
		syntax = true;
		editor.getSession().setMode({
			path: 'ace/mode/' + $scope.aceMode,
			v: Date.now()
		});
	}
	$scope.scrollTo = (type: string, name: string) => {
		var searchRegex = new RegExp("^\\s*" + type + "\\s+" + name, "g");
		editor.find(searchRegex, {
			backwards: false,
			wrap: true,
			caseSensitive: false,
			wholeWord: false,
			regExp: true,
		});
		if (type == "alert") { $scope.selectAlert(name); }
	}

	$scope.scrollToInterval = (id: string) => {
		document.getElementById('time-' + id).scrollIntoView();
		$scope.show($scope.sets[id]);
	};

	$scope.show = (set: any) => {
		set.show = 'loading...';
		$scope.animate();
		var url = '/api/rule?' +
			'alert=' + encodeURIComponent($scope.selected_alert) +
			'&from=' + encodeURIComponent(set.Time);
		$http.post(url, $scope.config_text)
			.success((data: any) => {
				procResults(data);
				set.Results = data.Sets[0].Results;
			})
			.error((error) => {
				$scope.error = error;
			})
			.finally(() => {
				$scope.stop();
				delete (set.show);
			});
	};

	$scope.getRunningHash = () => {
		if (!$scope.saveEnabled) {
			return
		}
		(function tick() {
			$http.get('/api/config/running_hash')
				.success((data: any) => {
					$scope.runningHashResult = '';
					$timeout(tick, 15 * 1000);
					if ($scope.runningHash) {
						if (data.Hash != $scope.runningHash) {
							$scope.runningChanged = true;
							return
						}
					}
					$scope.runningHash = data.Hash;
					$scope.runningChanged = false;
				})
				.error(function (data) {
					$scope.runningHashResult = "Error getting running config hash: " + data;
				})
		})()
	};

	$scope.getRunningHash();


	$scope.setInterval = () => {
		var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
		var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
		if (!from.isValid() || !to.isValid()) {
			return;
		}
		var diff = from.diff(to);
		if (!diff) {
			return;
		}
		var intervals = +$scope.intervals;
		if (intervals < 2) {
			return;
		}
		diff /= 1000 * 60;
		var d = Math.abs(Math.round(diff / intervals));
		if (d < 1) {
			d = 1;
		}
		$scope.duration = d;
	};
	$scope.setDuration = () => {
		var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
		var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
		if (!from.isValid() || !to.isValid()) {
			return;
		}
		var diff = from.diff(to);
		if (!diff) {
			return;
		}
		var duration = +$scope.duration;
		if (duration < 1) {
			return;
		}
		$scope.intervals = Math.abs(Math.round(diff / duration / 1000 / 60));
	};

	$scope.selectAlert = (alert: string) => {
		$scope.selected_alert = alert;
		$location.search("alert", alert);
		// Attempt to find `template = foo` in order to set up quick jump between template and alert
		var searchRegex = new RegExp("^\\s*alert\\s+" + alert, "g");
		var lines = $scope.config_text.split("\n");
		$scope.quickJumpTarget = null;
		for (var i = 0; i < lines.length; i++) {
			if (searchRegex.test(lines[i])) {
				for (var j = i + 1; j < lines.length; j++) {
					// Close bracket at start of line means end of alert.
					if (/^\s*\}/m.test(lines[j])) {
						return;
					}
					var found = /^\s*template\s*=\s*([\w\-\.\$]+)/m.exec(lines[j]);
					if (found) {
						$scope.quickJumpTarget = "template " + found[1];
					}
				}
			}
		}
	}

	$scope.quickJump = () => {
		var parts = $scope.quickJumpTarget.split(" ");
		if (parts.length != 2) { return; }
		$scope.scrollTo(parts[0], parts[1]);
		if (parts[0] == "template" && $scope.selected_alert) {
			$scope.quickJumpTarget = "alert " + $scope.selected_alert;
		}
	}

	$scope.setTemplateGroup = (group) => {
		var match = group.match(/{(.*)}/);
		if (match) {
			$scope.template_group = match[1];
		}
	}

	$scope.setNotificationToShow = (n:string)=>{
		$scope.notificationToShow = n;
	}
	
	var line_re = /test:(\d+)/;
	$scope.validate = () => {
		$http.post('/api/config_test', $scope.config_text)
			.success((data: any) => {
				if (data == "") {
					$scope.validationResult = "Valid";
					$timeout(() => {
						$scope.validationResult = "";
					}, 2000)
				} else {
					$scope.validationResult = data;
					var m = data.match(line_re);
					if (angular.isArray(m) && (m.length > 1)) {
						editor.gotoLine(m[1]);
					}
				}
			})
			.error((error) => {
				$scope.validationResult = 'Error validating: ' + error;
			});
	}

	$scope.test = () => {
		$scope.error = '';
		$scope.running = true;
		$scope.warning = [];
		$location.search('fromDate', $scope.fromDate || null);
		$location.search('fromTime', $scope.fromTime || null);
		$location.search('toDate', $scope.toDate || null);
		$location.search('toTime', $scope.toTime || null);
		$location.search('intervals', String($scope.intervals) || null);
		$location.search('duration', String($scope.duration) || null);
		$location.search('email', $scope.email || null);
		$location.search('template_group', $scope.template_group || null);
		$location.search('runningHash', $scope.runningHash)
		$location.search('runningChanged', $scope.runningChanged)
		$scope.animate();
		var from = moment.utc($scope.fromDate + ' ' + $scope.fromTime);
		var to = moment.utc($scope.toDate + ' ' + $scope.toTime);
		if (!from.isValid()) {
			from = to;
		}
		if (!to.isValid()) {
			to = from;
		}
		if (!from.isValid() && !to.isValid()) {
			from = to = moment.utc();
		}
		var diff = from.diff(to);
		var intervals;
		if (diff == 0) {
			intervals = 1;
		} else if (Math.abs(diff) < 60 * 1000) { // 1 minute
			intervals = 2;
		} else {
			intervals = +$scope.intervals;
		}
		var url = '/api/rule?' +
			'alert=' + encodeURIComponent($scope.selected_alert) +
			'&from=' + encodeURIComponent(from.format()) +
			'&to=' + encodeURIComponent(to.format()) +
			'&intervals=' + encodeURIComponent(intervals) +
			'&email=' + encodeURIComponent($scope.email) +
			'&incidentId=' + $scope.incidentId +
			'&template_group=' + encodeURIComponent($scope.template_group);
		$http.post(url, $scope.config_text)
			.success((data: any) => {
				$scope.sets = data.Sets;
				$scope.alert_history = data.AlertHistory;
				if (data.Hash) {
					$location.search('hash', data.Hash);
				}
				procResults(data);
			})
			.error((error) => {
				$scope.error = error;
			})
			.finally(() => {
				$scope.running = false;
				$scope.stop();
			});
	}

	$scope.zws = (v: string) => {
		return v.replace(/([,{}()])/g, '$1\u200b');
	};

	$scope.loadTimelinePanel = (entry: any, v: any) => {
		if (v.doneLoading && !v.error) { return; }
		v.error = null;
		v.doneLoading = false;
		var ak = entry.key;
		var openBrack = ak.indexOf("{");
		var closeBrack = ak.indexOf("}");
		var alertName = ak.substr(0, openBrack);
		var template = ak.substring(openBrack + 1, closeBrack);
		var url = '/api/rule?' +
			'alert=' + encodeURIComponent(alertName) +
			'&from=' + encodeURIComponent(moment.utc(v.Time).format()) +
			'&template_group=' + encodeURIComponent(template);
		$http.post(url, $scope.config_text)
			.success((data: any) => {
				v.subject = data.Subject;
				v.body = $sce.trustAsHtml(data.Body);
			})
			.error((error) => {
				v.error = error;
			})
			.finally(() => {
				v.doneLoading = true;
			});
	};

	function procResults(data: any) {
		$scope.subject = data.Subject;
		$scope.body = $sce.trustAsHtml(data.Body);
		if (data.EmailSubject) {
			data.EmailSubject = atob(data.EmailSubject)
		}
		$scope.emailSubject = data.EmailSubject
		if (data.EmailBody) {
			data.EmailBody = atob(data.EmailBody)
		}
		$scope.emailBody = $sce.trustAsHtml(data.EmailBody)
		$scope.customTemplates = {};
		for (var k in data.Custom) {
			$scope.customTemplates[k] = data.Custom[k];
		}
		var nots = {};
		_(data.Notifications).each((val,n)=>{
			if(val.Email){
				nots["Email "+ n] = val.Email;
			}
			if(val.Print != ""){
				nots["Print " +n] = {Print: val.Print};
			}
			_(val.HTTP).each((hp)=>{
				nots[hp.Method+" "+n] = hp;
			})
		})
		$scope.notifications = nots;
		var aNots = {};
		_(data.ActionNotifications).each((ts,n)=>{
			$scope.notificationToShow = "" + n;
			aNots[n] = {};
			_(ts).each((val,at)=>{
				if(val.Email){
					aNots[n]["Email ("+at+")"] = val.Email;
				}
				_(val.HTTP).each((hp)=>{
					aNots[n][hp.Method+" ("+at+")"] = hp;
				})
			})
		})

		$scope.actionNotifications = aNots;
		$scope.data = JSON.stringify(data.Data, null, '  ');
		$scope.error = data.Errors;
		$scope.warning = data.Warnings;
	}

	$scope.downloadConfig = () => {
		var blob = new Blob([$scope.config_text], { type: "text/plain;charset=utf-8" });
		saveAs(blob, "bosun.conf");
	}

	$scope.diffConfig = () => {
		$http.post('/api/config/diff',
			{
				"Config": $scope.config_text,
				"Message": $scope.message
			})
			.success((data: any) => {
				$scope.diff = data || "No Diff";
				// Reset running hash if there is no difference?
			})
			.error((error) => {
				$scope.diff = "Failed to load diff: " + error;
			});
	}


	$scope.saveConfig = () => {
		if (!$scope.saveEnabled) {
			return;
		}
		$scope.saveResult = "Saving; Please Wait"
		$http.post('/api/config/save', {
			"Config": $scope.config_text,
			"Diff": $scope.diff,
			"Message": $scope.message
		})
			.success((data: any) => {
				$scope.saveResult = "Config Saved; Reloading";
				$scope.runningHash = undefined;
			})
			.error((error) => {
				$scope.saveResult = error;
			});
	}

	$scope.saveClass = () => {
		if ($scope.saveResult == "Saving; Please Wait") {
			return "alert-warning"
		}
		if ($scope.saveResult == "Config Saved; Reloading") {
			return "alert-success"
		}
		return "alert-danger"
	}

	return $scope;
}]);

// declared in FileSaver.js
declare var saveAs: any;

class NotificationController {
	dat: any;
	test = () => {
		this.dat.msg = "sending"
		this.$http.post('/api/rule/notification/test', this.dat)
			.success((rDat: any) => {
				if (rDat.Error) {
					this.dat.msg = "Error: " + rDat.Error;
				} else {
					this.dat.msg = "Success! Status Code: " + rDat.Status;
				}
			})
			.error((error) => {
				this.dat.msg = "Error: " + error;
			});
	};
	static $inject = ['$http'];
    constructor(private $http: ng.IHttpService) {
    }
}

bosunApp.component('notification', {
	bindings: {
		dat: "<",
	},
	controller: NotificationController,
	controllerAs: 'ct',
	templateUrl : '/static/partials/notification.html',
});
