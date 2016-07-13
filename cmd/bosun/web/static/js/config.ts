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
	body: string;
	data: any;
	tab: string;
	zws: (v: string) => string;
	setTemplateGroup: (group: any) => void;
	scrollToInterval: (v: string) => void;
	show: (v: any) => void;
	loadTimelinePanel: (entry: any, v: any) => void;

	// saving
	user: string;
	message: string;
}

bosunControllers.controller('ConfigCtrl', ['$scope', '$http', '$location', '$route', '$timeout', '$sce', function ($scope: IConfigScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService, $timeout: ng.ITimeoutService, $sce: ng.ISCEService) {
	var search = $location.search();
	$scope.fromDate = search.fromDate || '';
	$scope.fromTime = search.fromTime || '';
	$scope.toDate = search.toDate || '';
	$scope.toTime = search.toTime || '';
	$scope.intervals = +search.intervals || 5;
	$scope.duration = +search.duration || null;
	$scope.config_text = 'Loading config...';
	$scope.selected_alert = search.alert || '';
	$scope.email = search.email || '';
	$scope.template_group = search.template_group || '';
	$scope.items = parseItems();
	$scope.tab = search.tab || 'results';
	$scope.aceTheme = 'chrome';
	$scope.aceMode = 'bosun';
	$scope.user = readCookie("action-user");


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
		$scope.data = JSON.stringify(data.Data, null, '  ');
		$scope.error = data.Errors;
		$scope.warning = data.Warnings;
	}

	$scope.downloadConfig = () => {
		var blob = new Blob([$scope.config_text], { type: "text/plain;charset=utf-8" });
		saveAs(blob, "bosun.conf");
	}

	$scope.saveConfig = () => {
		$scope.saveResult = "Saving; Please Wait"
		$http.post('/api/config/save',
			{
				"Config": $scope.config_text,
				"User": $scope.user,
				"Message": $scope.message
			})
			.success((data: any) => {
				createCookie("action-user", $scope.user, 1000);
				$scope.saveResult = "Config Saved; Reloading";
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