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
	template_group: string;
	animate: () => any;
	zws: (v: string) => string;
	test: () => any;
	scroll: (v: string) => void;
	intervals: number;
	duration: number;
	setInterval: () => void;
	setDuration: () => void;
	error: string;
	show: (v: any) => void;
	alert_history: any;
	running: boolean;
	loadAlert: (k: string) => void;
	assocations: any;
}

var tsdbFormat = 'YYYY/MM/DD-HH:mm';

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
	$scope.template_group = search.template_group || '';
	$scope.fromDate = search.fromDate || '';
	$scope.fromTime = search.fromTime || '';
	$scope.toDate = search.toDate || '';
	$scope.toTime = search.toTime || '';
	$scope.tab = search.tab || 'results';
	$scope.intervals = +search.intervals || 5;
	$scope.duration = +search.duration || null;
	if (!current_alert) {
		current_alert =
			'alert test {\n' +
			'	template = test\n' +
			'	crit = ' + expr + '\n' +
			'}';
	}
	$scope.alert = current_alert;
	try {
		current_template = atob(current_template);
	}
	catch (e) {
		current_template = '';
	}
	if (!current_template) {
		current_template =
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
	}
	$scope.template = current_template;
	$scope.shiftEnter = function($event: any) {
		if ($event.keyCode == 13 && $event.shiftKey) {
			$scope.test();
		}
	}
	$scope.loadAlert = function($selected: string) {
		$scope.alert = $scope.alerts[$selected];
		if (confirm('Load the associated notification template (will overwrite the current notification tempalte) ?')) {
			$scope.template = $scope.templates[$scope.assocations[$selected]];
		}
	}
	$scope.test = () => {
		$scope.error = '';
		$scope.running = true;
		$scope.warning = [];
		$location.search('alert', btoa($scope.alert));
		$location.search('template', btoa($scope.template));
		$location.search('fromDate', $scope.fromDate || null);
		$location.search('fromTime', $scope.fromTime || null);
		$location.search('toDate', $scope.toDate || null);
		$location.search('toTime', $scope.toTime || null);
		$location.search('tab', $scope.tab || 'results');
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
			'alert=' + encodeURIComponent($scope.alert) +
			'&template=' + encodeURIComponent($scope.template) +
			'&from=' + encodeURIComponent(from.format(tsdbFormat)) +
			'&to=' + encodeURIComponent(to.format(tsdbFormat)) +
			'&intervals=' + encodeURIComponent(intervals) +
			'&email=' + encodeURIComponent($scope.email) +
			'&template_group=' + encodeURIComponent($scope.template_group);
		$http.get(url)
			.success((data) => {
				$scope.sets = data.Sets;
				$scope.alert_history = data.AlertHistory;
				procResults(data);
			})
			.error((error) => {
				$scope.error = error;
			})
			.finally(() => {
				$scope.running = false;
				$scope.stop();
			});
	};
	function procResults(data: any) {
		$scope.subject = data.Subject;
		$scope.body = $sce.trustAsHtml(data.Body);
		$scope.data = JSON.stringify(data.Data, null, '  ');
		$scope.error = data.Errors;
		$scope.warning = data.Warnings;
	}
	$scope.show = (set: any) => {
		set.show = 'loading...';
		$scope.animate();
		var url = '/api/rule?' +
			'alert=' + encodeURIComponent($scope.alert) +
			'&template=' + encodeURIComponent($scope.template) +
			'&from=' + encodeURIComponent(set.Time);
		$http.get(url)
			.success((data) => {
				procResults(data);
				set.Results = data.Sets[0].Results;
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
	$scope.setInterval();
	$http.get('/api/templates')
		.success((data) => {
			$scope.alerts = data.Alerts;
			$scope.templates = data.Templates;
			$scope.assocations = data.Assocations;
		});
	$scope.test();
}]);
