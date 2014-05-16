interface ITestTemplateScope extends IExprScope {
	shiftEnter: ($event: any) => void;
	config: string;
	subject: string;
	body: string;
}

tsafControllers.controller('TestTemplateCtrl', ['$scope', '$http', '$location', '$route', function($scope: ITestTemplateScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	var search = $location.search();
	var current = search.config;
	try {
		current = atob(current);
	}
	catch (e) {
		current = '';
	}
	if (!current) {
		var def =
			'template test {\n' +
			'    body = `<h1>Name: {{.Alert.Name}}</h1>`\n' +
			'    subject = `{{.Last.Status}}: {{.Alert.Name}}: {{.E .Alert.Vars.q}} on {{.Group.host}}`\n' +
			'}\n\n' +
			'alert test {\n' +
			'    template = test\n' +
			'    $t = "5m"\n' +
			'    $q = avg(q("avg:rate{counter,,1}:os.cpu{host=*}", $t, ""))\n' +
			'    crit = $q > 10\n' +
			'}'
		$location.search('config', btoa(def));
		return;
	}
	$scope.config = current;
	$scope.running = "Running";
	$http.get('/api/template?' + 'config=' + encodeURIComponent($scope.config))
		.success((data) => {
			$scope.subject = data.Subject;
			$scope.body = data.Body;
			$scope.running = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$scope.shiftEnter = function($event: any) {
		if ($event.keyCode == 13 && $event.shiftKey) {
			$scope.set();
		}
	}
	$scope.set = () => {
		$location.search('config', btoa($scope.config));
		$route.reload();
	};
}]);