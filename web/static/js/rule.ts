interface IRuleScope extends IExprScope {
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
	$scope.fromDate = search.fromDate || '';
	$scope.fromTime = search.fromTime || '';
	$scope.toDate = search.toDate || '';
	$scope.toTime = search.toTime || '';
	$scope.tab = search.tab || 'results';
	$scope.intervals = +search.intervals || 5;
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
	$scope.test = () => {
		$scope.running = "Running";
		$scope.error = '';
		$scope.warning = [];
		$location.search('alert', btoa($scope.alert));
		$location.search('template', btoa($scope.template));
		$location.search('fromDate', $scope.fromDate || null);
		$location.search('fromTime', $scope.fromTime || null);
		$location.search('toDate', $scope.toDate || null);
		$location.search('toTime', $scope.toTime || null);
		$location.search('tab', $scope.tab || 'results');
		$location.search('intervals', $scope.intervals || null);
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
		function next(interval, first = false) {
			if (interval == 0) {
				$scope.stop();
				return;
			}
			var date = from.format('YYYY-MM-DD');
			var time = from.format('HH:mm');
			var url = '/api/rule?' +
				'alert=' + encodeURIComponent($scope.alert) +
				'&template=' + encodeURIComponent($scope.template) +
				'&date=' + encodeURIComponent(date) +
				'&time=' + encodeURIComponent(time);
			if (first) {
				url += '&notemplate=true';
			}
			$http.get(url)
				.success((data) => {
					if (first) {
						$scope.subject = data.Subject;
						$scope.body = $sce.trustAsHtml(data.Body);
						$scope.data = JSON.stringify(data.Data, null, '  ');
						angular.forEach(data.Warning, function(v) {
							$scope.warning.push(v)
						});
					}
					var results = [];
					var set: any = {
						time: moment.unix(data.Time).utc().format('YYYY-MM-DD HH:mm:ss'),
						critical: 0,
						warning: 0,
						normal: 0,
					};
					angular.forEach(data.Result, function(v, k) {
						results.push({
							group: k,
							result: v,
						})
						set[v.Status]++;
					});
					results.sort((a: any, b: any) => {
						return status_map[b.result.Status] - status_map[a.result.Status];
					});
					set.results = results;
					$scope.sets.push(set);
					$scope.running = '';
					$scope.error = '';
					from.subtract(diff / (intervals - 1));
					next(interval - 1);
				})
				.error((error) => {
					$scope.error = error;
					$scope.running = '';
				});
		}
		next(intervals, true);
	};
	$scope.zws = (v: string) => {
		return v.replace(/([,{}()])/g, '$1\u200b');
	};
	$scope.scroll = (id: string) => {
		document.getElementById('time-' + id).scrollIntoView();
	};
	$http.get('/api/templates')
		.success((data) => {
			$scope.alerts = data.Alerts;
			$scope.templates = data.Templates;
		});
	$scope.test();
}]);
