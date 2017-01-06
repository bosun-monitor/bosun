/// <reference path="0-bosun.ts" />
interface IAnnotationScope extends IBosunScope {
    id: string;
    annotation: Annotation;
    error: string;
    submitAnnotation: () => void;
    deleteAnnotation: () => void;
    owners: string[];
    hosts: string[];
    categories: string[];
    submitSuccess: boolean;
    deleteSuccess: boolean;
}

bosunControllers.controller('AnnotationCtrl', ['$scope', '$http', '$location', '$route', function ($scope: IAnnotationScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
    var search = $location.search();
    $scope.id = search.id;
    if ($scope.id && $scope.id != "") {
        $http.get('/api/annotation/' + $scope.id)
            .success((data: any) => {
                $scope.annotation = new Annotation(data, true);
                $scope.error = "";
            })
            .error((data: any) => {
                $scope.error = "failed to get annotation with id: " + $scope.id + ", error: " + data;
            });
    } else {
        $scope.annotation = new Annotation();
        $scope.annotation.setTimeUTC();
    }
    $http.get('/api/annotation/values/Owner')
        .success((data: string[]) => {
            $scope.owners = data;
        });
    $http.get('/api/annotation/values/Category')
        .success((data: string[]) => {
            $scope.categories = data;
        });
    $http.get('/api/annotation/values/Host')
        .success((data: string[]) => {
            $scope.hosts = data;
        });
    

    $scope.submitAnnotation = () => {
        $scope.animate();
        $scope.annotation.CreationUser = $scope.auth.GetUsername();
        $http.post('/api/annotation', $scope.annotation)
            .success((data: any) => {
                $scope.annotation = new Annotation(data, true);
                $scope.error = "";
                $scope.submitSuccess = true;
                $scope.deleteSuccess = false;
            })
            .error((error) => {
                $scope.error = "failed to create annotation: " + error.error;
                $scope.submitSuccess = false;
            })
            .finally(() => {
                $scope.stop();
            });
    };

    $scope.deleteAnnotation = () => {
        $scope.animate();
        $http.delete('/api/annotation/' + $scope.annotation.Id)
            .success((data) => {
                $scope.error = "";
                $scope.deleteSuccess = true;
                $scope.submitSuccess = false;
                $scope.annotation = new (Annotation);
                $scope.annotation.setTimeUTC();
            })
            .error((error) => {
                $scope.error = "failed to delete annotation with id: " + $scope.annotation.Id + ", error: " + error.error;
                $scope.deleteSuccess = false;
            })
            .finally(() => {
                $scope.stop();
            });
    }
}]);