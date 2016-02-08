interface IExprScope extends RootScope {
	expr: string;
	error: string;
	running: string;
	result: any;
	queries: any;
	result_type: string;
	set: () => void;
	tab: string;
	graph: any;
	bar: any;
	svg_url: string;
	date: string;
	time: string;
	keydown: ($event: any) => void;
	animate: () => any;
	stop: () => any;
}

bosunControllers.controller('ExprCtrl', ['$scope', '$http', '$location', '$route', function($scope: IExprScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	
	var search = $location.search();
	var current: string;
	try {
		current = atob(search.expr);
	}
	catch (e) {
		current = '';
	}
	if (!current) {
		$location.search('expr', btoa('avg(q("avg:rate:os.cpu{host=*bosun*}", "5m", "")) > 80'));
		return;
	}
	$scope.date = search.date || '';
	$scope.time = search.time || '';
	$scope.expr = current;

	$scope.running = current;
	$scope.tab = search.tab || 'results';
	$scope.animate();
	$http.post('/api/expr?' +
		'date=' + encodeURIComponent($scope.date) +
		'&time=' + encodeURIComponent($scope.time),current)
		.success((data) => {
			$scope.result = data.Results;
			$scope.queries = data.Queries;
			$scope.result_type = data.Type;
			if (data.Type == 'series') {
				$scope.svg_url = '/api/egraph/' + btoa(current) + '.svg?now=' + Math.floor(Date.now() / 1000);
				$scope.graph = toChart(data.Results);
			}
			if (data.Type == 'number') {
				 angular.forEach(data.Results, (d) => {
					var name = '{';
					angular.forEach(d.Group, (tagv, tagk) => {
							if (name.length > 1) {
								 name += ',';
							}
							name += tagk + '=' + tagv;
					});
					name += '}';
					d.name = name;
				 });
				$scope.bar = data.Results;
			}
			$scope.running = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		})
		.finally(() => {
			$scope.stop();
		});
	$scope.set = () => {
		$location.search('expr', btoa($scope.expr));
		$location.search('date', $scope.date || null);
		$location.search('time', $scope.time || null);
		$route.reload();
	};
	function toChart(res: any) {
		var graph: any = [];
		angular.forEach(res, (d, idx) => {
			var data: any = [];
			angular.forEach(d.Value, (val, ts) => {
				data.push([+ts, val]);
			});
			if (data.length == 0) {
				return;
			}
			var name = '{';
			angular.forEach(d.Group, (tagv, tagk) => {
				if (name.length > 1) {
					name += ',';
				}
				name += tagk + '=' + tagv;
			});
			name += '}';
			var series = {
				Data: data,
				Name: name,
			};
			graph[idx] = series;
		});
		return graph;
	}
	$scope.keydown = function($event: any) {
		if ($event.shiftKey && $event.keyCode == 13) {
			$scope.set();
		}
	};

}]);
