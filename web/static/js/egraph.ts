interface IEGraphScope extends ng.IScope {
	expr: string;
	error: string;
	warning: string;
	running: string;
	result: any;
	render: string;
	renderers: string[];
	bytes: boolean;
	url: string;
	set: () => void;
}

tsafControllers.controller('EGraphCtrl', ['$scope', '$http', '$location', '$route', function($scope: IEGraphScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	var search = $location.search();
	var current = search.q;
	try {
		current = atob(current);
	}
	catch (e) {}
	$scope.bytes = search.bytes == 'true';
	$scope.renderers = ['area', 'bar', 'line', 'scatterplot'];
	$scope.render = search.render || 'line';
	if (!current) {
		$location.search('q', btoa('q("avg:rate:os.cpu{host=ny-devtsaf01}", "5m", "")'));
		return;
	}
	$scope.expr = current;
	$scope.running = current;
	var width: number = $('.chart').width();
	var url = '/api/egraph?b64=' + btoa(current) + '&autods=' + width;
	$http.get(url)
		.success((data) => {
			$scope.result = data;
			if ($scope.result.length == 0) {
				$scope.warning = 'No Results';
			} else {
				$scope.warning = '';
			}
			$scope.running = '';
			$scope.error = '';
			$scope.url = url;
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$scope.set = () => {
		$location.search('q', btoa($scope.expr));
		$location.search('render', $scope.render);
		$location.search('bytes', $scope.bytes ? 'true' : undefined);
		$route.reload();
	};
}]);