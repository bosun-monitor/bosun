interface IRuleScope extends IBosunScope {
	shiftEnter: ($event: any) => void;
	alerts: any;
	templates: any;
	template: string;
	alert: string;
	tab: string;
	fromDate: string;
	toDate: string;
	fromTime: string;
	toTime: string;
	subject: string;
	email: string;
	body: string;
	warning: string[];
	sets: any;
	data: any;
	animate: () => any;
	stop: () => any;
	zws: (v: string) => string;
	test: () => any;
	scroll: (v: string) => void;
	intervals: number;
	duration: number;
	setInterval: () => void;
	setDuration: () => void;
	halt: () => void;
	stopped: boolean;
	remaining: number;
	error: string;
	show: (v: any) => void;
	alert_history: any;
}

bosunControllers.controller('RuleCtrl', ['$scope', '$http', '$location', '$route', '$sce', function($scope: IRuleScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService, $sce: ng.ISCEService) {
	var search = $location.search();
	var current_alert = atob(search.alert || '');
	var current_template = search.template;
	var expr = atob(search.expr || '') || 'avg(q("avg:rate{counter,,1}:os.cpu{host=*}", "5m", "")) > 10';
	var status_map: any = {
		"normal": 0,
		"warning": 1,
		"critical": 2,
	};
	$scope.email = search.email || '';
	$scope.fromDate = search.fromDate || '';
	$scope.fromTime = search.fromTime || '';
	$scope.toDate = search.toDate || '';
	$scope.toTime = search.toTime || '';
	$scope.tab = search.tab || 'results';
	$scope.intervals = +search.intervals || 5;
	$scope.duration = +search.duration || null;
	if (!current_alert) {
		var alert_def =
			'alert test {\n' +
			'	template = test\n' +
			'	crit = ' + expr + '\n' +
			'}';
		$location.search('alert', btoa(alert_def));
		$location.search('expr', null);
		return;
	}
	$scope.alert = current_alert;
	try {
		current_template = atob(current_template);
	}
	catch (e) {
		current_template = '';
	}
	if (!current_template) {
		var template_def =
			'template test {\n' +
			'	subject = {{.Last.Status}}: {{.Alert.Name}} on {{.Group.host}}\n' +
			'	body = `<p>Name: {{.Alert.Name}}\n' +
			'	<p>Tags:\n' +
			'	<table>\n' +
			'		{{range $k, $v := .Group}}\n' +
			'			<tr><td>{{$k}}</td><td>{{$v}}</td></tr>\n' +
			'		{{end}}\n' +
			'	</table>`\n' +
			'}';
		$location.search('template', btoa(template_def));
		return;
	}
	$scope.template = current_template;
	$scope.shiftEnter = function($event: any) {
		if ($event.keyCode == 13 && $event.shiftKey) {
			$scope.test();
		}
	}
	var alert_history = {};
	$scope.test = () => {
		$scope.error = '';
		$scope.stopped = false;
		$scope.warning = [];
		$location.search('alert', btoa($scope.alert));
		$location.search('template', btoa($scope.template));
		$location.search('fromDate', $scope.fromDate || null);
		$location.search('fromTime', $scope.fromTime || null);
		$location.search('toDate', $scope.toDate || null);
		$location.search('toTime', $scope.toTime || null);
		$location.search('tab', $scope.tab || 'results');
		$location.search('intervals', $scope.intervals || null);
		$location.search('duration', $scope.duration || null);
		$location.search('email', $scope.email || null);
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
			intervals = +($scope.intervals);
		}
		$scope.sets = [];
		var remaining = [];
		for (var i = 0; i < intervals; i++) {
			remaining.push(i);
		}
		var threads = Math.min(2, intervals);
		for (var i = 0; i < threads; i++) {
			next();
		}
		function next() {
			if (remaining.length == 0 || $scope.stopped) {
				threads--;
				if (threads) {
					return;
				}
				$scope.stop();
				$scope.remaining = 0;
				angular.forEach(alert_history, (v) => {
					var h = v.History;
					h.sort((a, b) => {
						var r = a.Time.isAfter(b.Time);
						return r ? 1 : -1;
					});
					angular.forEach(h, function(d: any, i: number) {
						if (i + 1 < h.length) {
							d.EndTime = h[i + 1].Time;
						} else {
							d.EndTime = d.Time;
						}
					});
					for (var i = 1; i < h.length; i++) {
						if (h[i].Status == h[i - 1].Status) {
							var r = h.splice(i, 1);
							h[i - 1].EndTime = r[0].EndTime;
							i--;
						}
					}
				});
				$scope.alert_history = alert_history;
				return;
			}
			$scope.remaining = remaining.length;
			var first = remaining.length == intervals;
			var interval = remaining.shift();
			var fromDate = from.clone();
			fromDate.subtract(diff / (intervals - 1) * interval);
			var date = fromDate.format('YYYY-MM-DD');
			var time = fromDate.format('HH:mm');
			var url = '/api/rule?' +
				'alert=' + encodeURIComponent($scope.alert) +
				'&template=' + encodeURIComponent($scope.template) +
				'&date=' + encodeURIComponent(date) +
				'&time=' + encodeURIComponent(time) +
				'&email=' + encodeURIComponent($scope.email);
			var f = first ? '' : '&summary=true';
			$http.get(url + f)
				.success((data) => {
					var set: any = {
						url: url,
						time: moment.unix(data.Time).utc().format('YYYY-MM-DD HH:mm:ss'),
						critical: data.Criticals.length,
						warning: data.Warnings.length,
						normal: data.Normals.length,
					};
					procHistory(data);
					if (first) {
						set.results = procResults(data);
					}
					$scope.sets.push(set);
					next();
				})
				.error((error) => {
					$scope.error = error;
					$scope.remaining = 0;
					$scope.stopped = true;
					threads = 0;
					remaining = [];
					$scope.stop();
				});
		}
	};
	function procHistory(data: any) {
		var t = moment.unix(data.Time).utc();
		function procStatus(st: string, d: any) {
			angular.forEach(d, function(v) {
				if (!alert_history[v]) {
					alert_history[v] = {History: []};
				}
				alert_history[v].History.push({
					Time: t,
					Status: st,
				});
			});
		}
		procStatus('critical', data.Criticals);
		procStatus('warning', data.Warnings);
		procStatus('normal', data.Normals);
	}
	function procResults(data: any) {
		$scope.subject = data.Subject;
		$scope.body = $sce.trustAsHtml(data.Body);
		$scope.data = JSON.stringify(data.Data, null, '  ');
		angular.forEach(data.Warning, function(v) {
			$scope.warning.push(v)
		});
		var results = [];
		angular.forEach(data.Result, function(v, k) {
			results.push({
				group: k,
				result: v,
			})
		});
		results.sort((a: any, b: any) => {
			return status_map[b.result.Status] - status_map[a.result.Status];
		});
		return results;
	}
	$scope.show = (set: any) => {
		set.show = 'loading...';
		$scope.animate();
		$http.get(set.url)
			.success((data) => {
				set.results = procResults(data);
			})
			.error((error) => {
				$scope.error = error;
			})
			.finally(() => {
				$scope.stop();
				delete(set.show);
			});
	};
	$scope.zws = (v: string) => {
		return v.replace(/([,{}()])/g, '$1\u200b');
	};
	$scope.scroll = (id: string) => {
		document.getElementById('time-' + id).scrollIntoView();
		$scope.show($scope.sets[id]);
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
	$scope.halt = () => {
		$scope.stopped = true;
	};
	$scope.setInterval();
	$http.get('/api/templates')
		.success((data) => {
			$scope.alerts = data.Alerts;
			$scope.templates = data.Templates;
		});
	$scope.test();
}]);
