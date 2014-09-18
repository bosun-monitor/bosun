interface IRuleScope extends IExprScope {
	shiftEnter: ($event: any) => void;
	alerts: any;
	templates: any;
	template: string;
	alert: string;
	tab: string;
	date: string;
	time: string;
	subject: string;
	body: string;
	warning: string[];
	results: any;
	resultTime: string;
	data: any;
	animate: () => any;
	stop: () => any;
	zws: (v: string) => string;
}

bosunControllers.controller('RuleCtrl', ['$scope', '$http', '$location', '$route', function($scope: IRuleScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	var current_alert = atob(search.alert || '');
	var current_template = search.template;
	var expr = atob(search.expr || '') || 'avg(q("avg:rate{counter,,1}:os.cpu{host=*}", "5m", "")) > 10';
	var status_map: any = {
		"normal": 0,
		"warning": 1,
		"critical": 2,
	}
	$scope.date = search.date || '';
	$scope.time = search.time || '';
	$scope.tab = search.tab || 'results';
	if (!current_alert) {
		var alert_def =
			'alert test {\n' +
			'	template = test\n' +
			'	crit = ' + expr + '\n' +
			'}'
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
			$scope.set();
		}
	}
	$scope.set = () => {
		$scope.running = "Running";
		$scope.error = '';
		$scope.warning = [];
		$location.search('alert', btoa($scope.alert));
		$location.search('template', btoa($scope.template));
		$location.search('date', $scope.date || null);
		$location.search('time', $scope.time || null);
		$location.search('tab', $scope.tab || 'results');
		$scope.animate();
		$http.get('/api/rule?' +
			'alert=' + encodeURIComponent($scope.alert) +
			'&template=' + encodeURIComponent($scope.template) +
			'&date=' + encodeURIComponent($scope.date) +
			'&time=' + encodeURIComponent($scope.time))
			.success((data) => {
				$scope.subject = data.Subject;
				$scope.body = data.Body;
				$scope.data = JSON.stringify(data.Data, null, '  ');
				$scope.resultTime = moment.unix(data.Time).utc().format('YYYY-MM-DD HH:mm:ss');
				$scope.results = [];
				angular.forEach(data.Result, function(v, k) {
					$scope.results.push({
						group: k,
						result: v,
					})
				});
				$scope.results.sort((a: any, b: any) => {
					return status_map[b.result.Status] - status_map[a.result.Status];
				});
				angular.forEach(data.Warning, function(v) {
					$scope.warning.push(v)
				});
				$scope.running = '';
				$scope.error = '';
			})
			.error((error) => {
				$scope.error = error;
				$scope.running = '';
			})
			.finally(() => {
				$scope.stop();
			});
	};
	$scope.zws = (v: string) => {
		return v.replace(/([,{}()])/g, '$1\u200b');
	};
	$http.get('/api/templates')
		.success((data) => {
			$scope.alerts = data.Alerts;
			$scope.templates = data.Templates;
		});
	$scope.set();
}]);
