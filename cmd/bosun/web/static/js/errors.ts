interface IErrorScope extends IBosunScope {
	errors: any;
	error: string;
	loading: boolean;
	totalLines: () => number;
	selectedLines: ()=> number;
	check: (err: any) => void;
	click: (err: any, event:any) => void;
	clearAll: () => void;
	clearSelected: () => void;
	ruleLink: (line:any, err:any) => string;
}

bosunControllers.controller('ErrorCtrl', ['$scope', '$http', '$location', '$route', function($scope: IErrorScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	$scope.loading = true
	$http.get('/api/errors')
		.success((data: any) => {
			$scope.errors = [];
			_(data).forEach((err,name)=>{
				err.Name = name;
				err.Shown = false;
                err.Sum = err.Errors.Count;
			    err.Errors.FirstTime = moment.utc(err.Errors.FirstTime);
				err.Errors.LastTime = moment.utc(err.Errors.LastTime);
				$scope.errors.push(err);
			})
		})
		.error(function(data) {
   			$scope.error = "Error fetching data: " + data;
  		})
		.finally(()=>{$scope.loading=false})
	
	
	$scope.click = (err, event) => {
		event.stopPropagation();
	};
	
	$scope.totalLines = () => {
        if (typeof $scope.errors === 'undefined') {
            return -1;
        };
		return $scope.errors.length;
	};
	
	$scope.selectedLines = () => {
		var t = 0;
		_($scope.errors).forEach((err) =>{
			if (err.checked){
				t++;
			}
		})
		return t;
	};
	
	var getChecked = () => {
		var keys = [];
		_($scope.errors).forEach((err) =>{
			if (err.checked){
				keys.push(err.Name)
			}
		})
		return keys;
	}
	
	var clear = (keys) => {
		$http.post('/api/errors', keys)
		.success((data) => {
			$route.reload();
		})
		.error(function(data) {
   			$scope.error = "Error Clearing Errors: " + data;
  		})
	}
	
	$scope.clearAll = () =>{
		clear(["all"]);
	}
	
	$scope.clearSelected = () => {
		var keys = getChecked();
		clear(keys);
	}
	
	$scope.ruleLink = (line,err) => {
		var url = "/config?alert=" + err.Name;
		var fromDate = moment.utc(line.FirstTime)
		url += "&fromDate=" + fromDate.format("YYYY-MM-DD");
		url += "&fromTime=" + fromDate.format("hh:mm")
		var toDate = moment.utc(line.LastTime)
		url += "&toDate=" + toDate.format("YYYY-MM-DD");
		url += "&toTime=" + toDate.format("hh:mm")
		return url;
	}
}]);
