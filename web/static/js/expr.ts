interface IExprScope extends ng.IScope {
	expr: string;
	error: string;
	running: string;
	result: any;
	queries: any;
	result_type: string;
	set: () => void;
}

tsafControllers.controller('ExprCtrl', ['$scope', '$http', '$location', '$route', function($scope: IExprScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var current = $location.hash();
	try {
		current = atob(current);
	}
	catch (e) {
		current = '';
	}
	if (!current) {
		$location.hash(btoa('avg(q("avg:rate:os.cpu{host=ny-devtsaf01}", "5m", "")) > 80'));
		return;
	}
	$scope.expr = current;
	$scope.running = current;
	$http.get('/api/expr?q=' + encodeURIComponent(current))
		.success((data) => {
			$scope.result = data.Results;
			$scope.queries = data.Queries;
			$scope.result_type = data.Type;
			$scope.running = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$scope.set = () => {
		$location.hash(btoa($scope.expr));
		$route.reload();
	};
}]);