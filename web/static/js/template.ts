interface ITestTemplateScope extends IExprScope {
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
}

tsafControllers.controller('TestTemplateCtrl', ['$scope', '$http', '$location', '$route', function($scope: ITestTemplateScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	var current_alert = search.alert;
	var current_template = search.template;
	var status_map: any = {
		"Ok": 0,
		"Warn": 1,
		"Crit": 2,
	}
	$scope.date = search.date || '';
	$scope.time = search.time || '';
	$scope.tab = search.tab || 'results';
	$scope.selected_alert = '';
	$http.get('/api/config/alerts')
			.success((data) => {
				$scope.alerts = data;
			})
	$http.get('/api/config/templates')
			.success((data) => {
				$scope.templates = data;
			})
	try {
		current_alert = atob(current_alert);
	}
	catch (e) {
		current_alert = '';
	}
	if (!current_alert) {
		var alert_def =
			'alert test {\n' +
			'    template = test\n' +
			'    $t = "5m"\n' +
			'    $q = avg(q("avg:rate{counter,,1}:os.cpu{host=*}", $t, ""))\n' +
			'    crit = $q > 10\n' +
			'}'
		$location.search('alert', btoa(alert_def));
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
			'    body = `<h1>Name: {{.Alert.Name}}</h1>`\n' +
			'    subject = `{{.Last.Status}}: {{.Alert.Name}}: {{.E .Alert.Vars.q}} on {{.Group.host}}`\n' +
			'}'
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
		$location.search('alert', btoa($scope.alert));
		$location.search('template', btoa($scope.template));
		$location.search('date', $scope.date || null);
		$location.search('time', $scope.time || null);
		$location.search('tab', $scope.tab || 'results');
		$http.get('/api/template?' +
			'alert=' + encodeURIComponent($scope.alert) +
			'&template=' + encodeURIComponent($scope.template) +
			'&date=' + encodeURIComponent($scope.date) +
			'&time=' + encodeURIComponent($scope.time))
			.success((data) => {
				$scope.subject = data.Subject;
				$scope.body = data.Body;
				$scope.result = data.Result;
				angular.forEach($scope.result, function(v) {
					v.status_number = status_map[v.Status]
				});
				$scope.running = '';
			})
			.error((error) => {
				$scope.error = error;
				$scope.running = '';
			});
	};
	$scope.set();
}]);