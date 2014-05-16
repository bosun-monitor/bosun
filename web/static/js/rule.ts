interface IRuleScope extends IExprScope {
	date: string;
	time: string;
	shiftEnter: ($event: any) => void;
}

tsafControllers.controller('RuleCtrl', ['$scope', '$http', '$location', '$route', function($scope: IRuleScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	var current = search.rule;
	$scope.date = search.date || '';
	$scope.time = search.time || '';
	try {
		current = atob(current);
	}
	catch (e) {
		current = '';
	}
	if (!current) {
		var def = '$t = "5m"\n' +
			'crit = avg(q("avg:os.cpu", $t, "")) > 10';
		$location.search('rule', btoa(def));
		return;
	}
	$scope.expr = current;
	$scope.running = current;
	$http.get('/api/rule?' +
		'rule=' + encodeURIComponent(current) +
		'&date=' + encodeURIComponent($scope.date) +
		'&time=' + encodeURIComponent($scope.time))
		.success((data) => {
			$scope.result = data.Results;
			$scope.queries = data.Queries;
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
		$location.search('rule', btoa($scope.expr));
		$location.search('date', $scope.date || null);
		$location.search('time', $scope.time || null);
		$route.reload();
	};
}]);
