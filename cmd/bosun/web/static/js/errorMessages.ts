interface IErrorMessagesScope extends IBosunScope {
	errors: any;
	error: string;
	loading: boolean;
	totalLines: () => number;
	click: (err: any, event:any) => void;
    hideShow: (err: any, event:any) => void;
	clearAll: (key: string) => void;
}

bosunControllers.controller('ErrorMessagesCtrl', ['$scope', '$http', '$location', '$route', function($scope: IErrorMessagesScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
    var search = $location.search();
    $scope.alert_key = search.key
	$scope.loading = true
	$http.get('/api/errors?key='+search.key)
		.success((data: any) => {
			_(data).forEach((err,name)=>{
				err.Name = name;
                err.Sum = 0;
                _(err.Errors).forEach((line)=>{
                    line.Shown = true
                    err.Sum += line.Count
                    line.FirstTime = moment.utc(line.FirstTime)
                    line.LastTime = moment.utc(line.LastTime)
                })
				$scope.errors = err;
			})
		})
		.error(function(data) {
   			$scope.error = "Error fetching data: " + data;
  		})
		.finally(()=>{$scope.loading=false})
	
	
	$scope.click = (err, event) => {
		event.stopPropagation();
	};
	
	$scope.hideShow = (err, event) => {
        err.Shown = !err.Shown;
	};

    $scope.totalLines = () => {
        if (typeof $scope.errors === 'undefined') {
            return -1;
        };
        return $scope.errors.length;
    };
	
	$scope.clearAll = (key) => {
		$http.post('/api/errors', [key])
		.success((data) => {
            $route.reload();
		})
		.error(function(data) {
   			$scope.error = "Error Clearing Errors: " + data;
  		})
	}
}]);
