interface IConfigScope extends ng.IScope {
	current: string;
	result: string;
	error: string;
	config_text: string;
	editorOptions: any;
	editor: any;
	codemirrorLoaded: (editor: any) => void;
	set: () => void;
	line: number;
}

tsafControllers.controller('ConfigCtrl', ['$scope', '$http', '$location', '$route', function($scope: IConfigScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	var current = search.config_text;
	var line_re = /test:(\d+)/;
	try {
		current = atob(current);
	}
	catch (e) {
		current = '';
	}
	if (!current) {
		var def = '';
		$http.get('/api/config')
			.success((data) => {
				def = data;
			})
			.finally(() => {
				$location.search('config_text', btoa(def));
			});
		return;
	}
	$scope.config_text = current;
	$scope.set = () => {
		$scope.result = null;
		$scope.line = null;
		$http.get('/api/config_test?config_text=' + encodeURIComponent($scope.config_text))
			.success((data) => {
				if (data == "") {
					$scope.result = "Valid";
				} else {
					$scope.result = data;
					var m = data.match(line_re);
					if (angular.isArray(m) && (m.length > 1)) {
						$scope.line = m[1];
					}
				}
			})
			.error((error) => {
				$scope.error = error || 'Error';
			});
	}
	$scope.set();
}]);