/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="angular-sanitize.d.ts" />
/// <reference path="bootstrap.d.ts" />
/// <reference path="moment.d.ts" />
/// <reference path="d3.d.ts" />
var tsafApp = angular.module('tsafApp', [
    'ngRoute',
    'tsafControllers',
    'mgcrea.ngStrap',
    'ngSanitize'
]);

tsafApp.config([
    '$routeProvider', '$locationProvider', function ($routeProvider, $locationProvider) {
        $locationProvider.html5Mode(true);
        $routeProvider.when('/', {
            title: 'Dashboard',
            templateUrl: 'partials/dashboard.html',
            controller: 'DashboardCtrl'
        }).when('/items', {
            title: 'Items',
            templateUrl: 'partials/items.html',
            controller: 'ItemsCtrl'
        }).when('/expr', {
            title: 'Expression',
            templateUrl: 'partials/expr.html',
            controller: 'ExprCtrl'
        }).when('/graph', {
            title: 'Graph',
            templateUrl: 'partials/graph.html',
            controller: 'GraphCtrl'
        }).when('/host', {
            title: 'Host View',
            templateUrl: 'partials/host.html',
            controller: 'HostCtrl',
            reloadOnSearch: false
        }).when('/rule', {
            title: 'Rule',
            templateUrl: 'partials/rule.html',
            controller: 'RuleCtrl'
        }).when('/silence', {
            title: 'Silence',
            templateUrl: 'partials/silence.html',
            controller: 'SilenceCtrl'
        }).when('/config', {
            title: 'Configuration',
            templateUrl: 'partials/config.html',
            controller: 'ConfigCtrl'
        }).when('/action', {
            title: 'Action',
            templateUrl: 'partials/action.html',
            controller: 'ActionCtrl'
        }).when('/history', {
            title: 'Alert History',
            templateUrl: 'partials/history.html',
            controller: 'HistoryCtrl'
        }).otherwise({
            redirectTo: '/'
        });
    }]);

tsafApp.run([
    '$location', '$rootScope', function ($location, $rootScope) {
        $rootScope.$on('$routeChangeSuccess', function (event, current, previous) {
            $rootScope.title = current.$$route.title;
        });
    }]);

var tsafControllers = angular.module('tsafControllers', []);

tsafControllers.controller('TsafCtrl', [
    '$scope', '$route', '$http', function ($scope, $route, $http) {
        $scope.active = function (v) {
            if (!$route.current) {
                return null;
            }
            if ($route.current.loadedTemplateUrl == 'partials/' + v + '.html') {
                return { active: true };
            }
            return null;
        };
        $scope.json = function (v) {
            return JSON.stringify(v, null, '  ');
        };
        $scope.btoa = function (v) {
            return btoa(v);
        };
        $scope.encode = function (v) {
            return encodeURIComponent(v);
        };
        $scope.req_from_m = function (m) {
            var r = new Request();
            var q = new Query();
            q.metric = m;
            r.queries.push(q);
            return r;
        };
        $scope.panelClass = function (status) {
            switch (status) {
                case "critical":
                    return "panel-danger";
                case "unknown":
                    return "panel-info";
                case "warning":
                    return "panel-warning";
                case "normal":
                    return "panel-success";
                default:
                    return "panel-default";
            }
        };
        $scope.refresh = function (cb) {
            $http.get('/api/alerts').success(function (data) {
                angular.forEach(data.Status, function (v, k) {
                    v.Touched = moment(v.Touched).utc();
                    angular.forEach(v.History, function (v, k) {
                        v.Time = moment(v.Time).utc();
                    });
                    v.last = v.History[v.History.length - 1];
                });
                $scope.schedule = data;
                $scope.timeanddate = data.TimeAndDate;
                if (cb) {
                    cb();
                }
            });
        };
    }]);

moment.defaultFormat = 'YYYY/MM/DD-HH:mm:ss';

moment.lang('en', {
    relativeTime: {
        future: "in %s",
        past: "%s-ago",
        s: "%ds",
        m: "%dm",
        mm: "%dm",
        h: "%dh",
        hh: "%dh",
        d: "%dd",
        dd: "%dd",
        M: "%dn",
        MM: "%dn",
        y: "%dy",
        yy: "%dy"
    }
});
tsafControllers.controller('ActionCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.type = search.type;
        if (!angular.isArray(search.key)) {
            $scope.keys = [search.key];
        } else {
            $scope.keys = search.key;
        }
        $scope.submit = function () {
            var data = {
                Type: $scope.type,
                User: $scope.user,
                Message: $scope.message,
                Keys: $scope.keys
            };
            $http.post('/api/action', data).success(function (data) {
                $location.url('/');
            }).error(function (error) {
                alert(error);
            });
        };
    }]);
tsafControllers.controller('ConfigCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var current = search.config_text;
        var line_re = /test:(\d+)/;
        try  {
            current = atob(current);
        } catch (e) {
            current = '';
        }
        if (!current) {
            var def = '';
            $http.get('/api/config').success(function (data) {
                def = data;
            }).finally(function () {
                $location.search('config_text', btoa(def));
            });
            return;
        }
        $scope.config_text = current;
        $scope.set = function () {
            $scope.result = null;
            $scope.line = null;
            $http.get('/api/config_test?config_text=' + encodeURIComponent($scope.config_text)).success(function (data) {
                if (data == "") {
                    $scope.result = "Valid";
                } else {
                    $scope.result = data;
                    var m = data.match(line_re);
                    if (angular.isArray(m) && (m.length > 1)) {
                        $scope.line = m[1];
                    }
                }
            }).error(function (error) {
                $scope.error = error || 'Error';
            });
        };
        $scope.set();
    }]);
tsafControllers.controller('DashboardCtrl', [
    '$scope', function ($scope) {
        $scope.refresh();
    }]);
tsafApp.directive('tsResults', function () {
    return {
        templateUrl: '/partials/results.html'
    };
});

var timeFormat = 'YYYY-MM-DD HH:mm:ss ZZ';

tsafApp.directive("tsTime", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsTime, function (v) {
                var m = moment(v).utc();
                var text = m.format(timeFormat) + ' (' + m.fromNow() + ')';
                if (attrs.noLink) {
                    elem.text(m.format(timeFormat) + ' (' + m.fromNow() + ')');
                } else {
                    var el = document.createElement('a');
                    el.innerText = text;
                    el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
                    el.href += m.format('YYYYMMDDTHHmm');
                    el.href += '&p1=0';
                    angular.forEach(scope.timeanddate, function (v, k) {
                        el.href += '&p' + (k + 2) + '=' + v;
                    });
                    elem.html(el);
                }
            });
        }
    };
});

tsafApp.directive("tsSince", function () {
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.tsSince, function (v) {
                var m = moment(v).utc();
                elem.text(m.fromNow());
            });
        }
    };
});

tsafApp.directive("tooltip", function () {
    return {
        link: function (scope, elem, attrs) {
            angular.element(elem[0]).tooltip({ placement: "bottom" });
        }
    };
});

tsafApp.directive('tsLine', function () {
    return {
        link: function (scope, elem, attrs) {
            elem.linedtextarea();
            var parent = elem.parent();
            var linesDiv = parent;
            function lineHighlight(line) {
                var lineHeight = elem[0].scrollHeight / (elem[0].value.match(/\n/g).length + 1);
                var jump = (line - 1) * lineHeight;
                elem.scrollTop(jump);
                elem.scroll();
                parent.find('.lines div').eq(line - 1).addClass('lineerror');
            }
            function lineClear() {
                parent.find('.lineerror').removeClass('lineerror');
            }
            scope.$watch(attrs.tsLine, function (v) {
                lineClear();
                if (v) {
                    lineHighlight(v);
                }
            });
        }
    };
});

tsafApp.directive('tsTableSort', [
    '$timeout', function ($timeout) {
        return {
            link: function (scope, elem, attrs) {
                $timeout(function () {
                    $(elem).tablesorter({
                        sortList: scope.$eval(attrs.tsTableSort)
                    });
                });
            }
        };
    }]);

tsafApp.directive('ahTimeLine', function () {
    //2014-05-26T21:46:37.435056942Z
    var format = d3.time.format.utc("%Y-%m-%dT%X");
    var tsdbFormat = d3.time.format.utc("%Y/%m/%d-%X");
    function parseDate(s) {
        return s.toDate();
    }
    var margin = {
        top: 20,
        right: 80,
        bottom: 30,
        left: 250
    };
    var customTimeFormat = d3.time.format.utc.multi([
        [".%L", function (d) {
                return d.getMilliseconds();
            }],
        [":%S", function (d) {
                return d.getSeconds();
            }],
        ["%H:%M", function (d) {
                return d.getMinutes();
            }],
        ["%H", function (d) {
                return d.getHours();
            }],
        ["%a %d", function (d) {
                return d.getDay() && d.getDate() != 1;
            }],
        ["%b %d", function (d) {
                return d.getDate() != 1;
            }],
        ["%B", function (d) {
                return d.getMonth();
            }],
        ["%Y", function () {
                return true;
            }]
    ]);
    return {
        link: function (scope, elem, attrs) {
            scope.$watch(attrs.data, update);
            function update(v) {
                if (!angular.isArray(v) || v.length == 0) {
                    return;
                }
                var svgHeight = v.length * 15 + margin.top + margin.bottom;
                var height = svgHeight - margin.top - margin.bottom;
                var svgWidth = elem.width();
                var width = svgWidth - margin.left - margin.right;
                var xScale = d3.time.scale.utc().range([0, width]);
                var yScale = d3.scale.linear().range([height, 0]);
                var xAxis = d3.svg.axis().scale(xScale).tickFormat(customTimeFormat).orient('bottom');
                var chart = d3.select(elem[0]).append('svg').attr('width', svgWidth).attr('height', svgHeight).append('g').attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                chart.append('g').attr('class', 'x axis').attr('transform', 'translate(0,' + height + ')');
                chart.append('g').attr('class', 'y axis');
                var legend = d3.select(elem[0]).append('div').attr('class', 'legend');
                var time_legend = legend.append('div').text(tsdbFormat(new Date()));
                var alert_legend = legend.append('div').text('Alert');
                var max_date = new Date(-8640000000000000);
                var min_date = new Date(8640000000000000);
                v.forEach(function (a) {
                    a.History.forEach(function (d) {
                        if (parseDate(d.Time) < min_date) {
                            min_date = parseDate(d.Time);
                        }
                        if (parseDate(d.EndTime) > max_date) {
                            max_date = parseDate(d.EndTime);
                        }
                    });
                });
                xScale.domain([min_date, max_date]);
                yScale.domain([0, v.length]);
                chart.select('.x.axis').transition().call(xAxis);
                v.forEach(function (a, i) {
                    chart.selectAll('.bars').data(a.History).enter().append('rect').attr('class', function (d) {
                        return d.Status;
                    }).attr('x', function (d) {
                        return xScale(parseDate(d.Time));
                    }).attr('y', yScale(i + 1)).attr('height', height - yScale(.95)).attr('width', function (d) {
                        return xScale(parseDate(d.EndTime)) - xScale(parseDate(d.Time));
                    }).on('mousemove.x', mousemove_x).on('mousemove.y', function (d) {
                        alert_legend.text(a.Name);
                    }).on('click', function (d, j) {
                        var id = 'panel' + i + '-' + j;
                        scope.$apply(scope.shown['group' + i] = true);
                        scope.$apply(scope.shown[id] = true);
                        $('html, body').scrollTop($("#" + id).offset().top);
                    });
                });
                chart.selectAll('.labels').data(v).enter().append('text').attr('text-anchor', 'end').attr('x', 0).attr('dx', '-.5em').attr('dy', '.25em').attr('y', function (d, i) {
                    return yScale(i) - (height - yScale(.5));
                }).text(function (d) {
                    return d.Name;
                });
                chart.selectAll('.sep').data(v).enter().append('rect').attr('y', function (d, i) {
                    return yScale(i) - (height - yScale(.05));
                }).attr('height', function (d, i) {
                    return (height - yScale(.05));
                }).attr('x', 0).attr('width', width).on('mousemove.x', mousemove_x);
                function mousemove_x() {
                    var x = xScale.invert(d3.mouse(this)[0]);
                    time_legend.text(tsdbFormat(x));
                }
            }
            ;
        }
    };
});

var fmtUnits = ['', 'k', 'M', 'G', 'T', 'P', 'E'];

function nfmt(s, mult, suffix, opts) {
    opts = opts || {};
    var n = parseFloat(s);
    if (opts.round)
        n = Math.round(n);
    if (!n)
        return suffix ? '0 ' + suffix : '0';
    if (isNaN(n) || !isFinite(n))
        return '-';
    var a = Math.abs(n);
    var precision = a < 1 ? 2 : 4;
    if (a >= 1) {
        var number = Math.floor(Math.log(a) / Math.log(mult));
        a /= Math.pow(mult, Math.floor(number));
        if (fmtUnits[number]) {
            suffix = fmtUnits[number] + suffix;
        }
    }
    if (n < 0)
        a = -a;
    var r = a.toFixed(precision);
    return r + suffix;
}

tsafApp.filter('nfmt', function () {
    return function (s) {
        return nfmt(s, 1000, '', {});
    };
});

tsafApp.filter('bytes', function () {
    return function (s) {
        return nfmt(s, 1024, 'B', { round: true });
    };
});

tsafApp.filter('bits', function () {
    return function (s) {
        return nfmt(s, 1024, 'b', { round: true });
    };
});

tsafApp.directive('tsGraph', [
    '$window', 'nfmtFilter', function ($window, fmtfilter) {
        var margin = {
            top: 10,
            right: 10,
            bottom: 30,
            left: 80
        };
        return {
            scope: {
                data: '=',
                height: '=',
                generator: '='
            },
            link: function (scope, elem, attrs) {
                var svgHeight = +scope.height || 150;
                var height = svgHeight - margin.top - margin.bottom;
                var svgWidth;
                var width;
                var yScale = d3.scale.linear().range([height, 0]);
                var xScale;
                var xAxis = d3.svg.axis().orient('bottom');
                var yAxis = d3.svg.axis().scale(yScale).orient('left').ticks(Math.min(10, height / 20)).tickFormat(fmtfilter);
                var line;
                switch (scope.generator) {
                    case 'area':
                        line = d3.svg.area();
                        break;
                    default:
                        line = d3.svg.line();
                }
                line.y(function (d) {
                    return yScale(d.y);
                });
                line.x(function (d) {
                    return xScale(d.x * 1000);
                });
                var svg = d3.select(elem[0]).append('svg').attr('height', svgHeight).append('g').attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                var defs = svg.append('defs').append('clipPath').attr('id', 'clip').append('rect').attr('height', height);
                var chart = svg.append('g').attr('pointer-events', 'all').attr('clip-path', 'url(#clip)');
                svg.append('g').attr('class', 'x axis').attr('transform', 'translate(0,' + height + ')');
                svg.append('g').attr('class', 'y axis');
                var xloc = d3.select(elem[0]).append('div');
                var legend = d3.select(elem[0]).append('div');
                var color = d3.scale.category10();
                var mousex = 0;
                var oldx = 0;
                var data;
                var focus = svg.append('g').attr('class', 'focus');
                focus.append('line');
                function mouseover() {
                    var pt = d3.mouse(this);
                    mousex = pt[0];
                    if (data) {
                        drawLegend();
                    }
                }
                function drawLegend() {
                    var names = legend.selectAll('.series').data(data, function (d) {
                        return d.name;
                    });
                    names.enter().append('div').attr('class', 'series');
                    names.exit().remove();
                    var xi = xScale.invert(mousex);
                    xloc.text('Time: ' + moment(xi).utc().format());
                    var t = xi.getTime() / 1000;
                    names.text(function (d) {
                        var idx = bisect(d.data, t);
                        if (idx >= d.data.length) {
                            idx = d.data.length - 1;
                        }
                        var pt = d.data[idx];
                        if (pt) {
                            return d.name + ': ' + pt.y;
                        }
                    }).style('color', function (d) {
                        return color(d.name);
                    });
                    var x = mousex;
                    if (x > width) {
                        x = 0;
                    }
                    focus.select('line').attr('x1', x).attr('x2', x).attr('y1', 0).attr('y2', height);
                }
                scope.$watch('data', update);
                var w = angular.element($window);
                scope.$watch(function () {
                    return w.width();
                }, resize, true);
                w.bind('resize', function () {
                    scope.$apply();
                });
                function resize() {
                    svgWidth = elem.width();
                    width = svgWidth - margin.left - margin.right;
                    xScale = d3.time.scale.utc().range([0, width]);
                    xAxis.scale(xScale);
                    if (!mousex) {
                        mousex = width + 1;
                    }
                    svg.attr('width', svgWidth);
                    defs.attr('width', width);
                    xAxis.ticks(width / 60);
                    draw();
                    chart.selectAll('rect.click-capture').remove();
                    chart.append('rect').attr('class', 'click-capture').style('visibility', 'hidden').attr('x', 0).attr('y', 0).attr('height', height).attr('width', width).on('mousemove', mouseover);
                }
                var oldx = 0;
                var bisect = d3.bisector(function (d) {
                    return d.x;
                }).right;
                function update(v) {
                    if (!angular.isArray(v) || v.length == 0) {
                        return;
                    }
                    data = v;
                    resize();
                }
                function draw() {
                    if (!data || !xScale) {
                        return;
                    }
                    var xdomain = [
                        d3.min(data, function (d) {
                            return d3.min(d.data, function (c) {
                                return c.x;
                            });
                        }) * 1000,
                        d3.max(data, function (d) {
                            return d3.max(d.data, function (c) {
                                return c.x;
                            });
                        }) * 1000
                    ];
                    if (!oldx) {
                        oldx = xdomain[1];
                    } else if (oldx == xdomain[1]) {
                        return;
                    }
                    xScale.domain(xdomain);
                    yScale.domain([
                        d3.min(data, function (d) {
                            return d3.min(d.data, function (c) {
                                return c.y;
                            });
                        }),
                        d3.max(data, function (d) {
                            return d3.max(d.data, function (c) {
                                return c.y;
                            });
                        })
                    ]);
                    if (scope.generator == 'area') {
                        line.y0(yScale(0));
                    }
                    svg.select('.x.axis').transition().call(xAxis);
                    svg.select('.y.axis').transition().call(yAxis);
                    var queries = chart.selectAll('.line').data(data, function (d) {
                        return d.name;
                    });
                    switch (scope.generator) {
                        case 'area':
                            queries.enter().append('path').attr('stroke', function (d) {
                                return color(d.name);
                            }).attr('class', 'line').style('fill', function (d) {
                                return color(d.name);
                            });
                            break;
                        default:
                            queries.enter().append('path').attr('stroke', function (d) {
                                return color(d.name);
                            }).attr('class', 'line');
                    }
                    queries.exit().remove();
                    queries.attr('d', function (d) {
                        return line(d.data);
                    }).attr('transform', null).transition().ease('linear').attr('transform', 'translate(' + (xScale(oldx) - xScale(xdomain[1])) + ')');
                    oldx = xdomain[1];
                    drawLegend();
                }
                ;
            }
        };
    }]);
tsafControllers.controller('ExprCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var current = $location.hash();
        try  {
            current = atob(current);
        } catch (e) {
            current = '';
        }
        if (!current) {
            $location.hash(btoa('avg(q("avg:rate:os.cpu{host=ny-devtsaf01}", "5m", "")) > 80'));
            return;
        }
        $scope.expr = current;
        $scope.running = current;
        $scope.tab = 'results';
        $http.get('/api/expr?q=' + encodeURIComponent(current)).success(function (data) {
            $scope.result = data.Results;
            $scope.queries = data.Queries;
            $scope.result_type = data.Type;
            if (data.Type == 'series') {
                $scope.svg_url = '/api/egraph/' + btoa(current) + '.svg?now=' + Math.floor(Date.now() / 1000);
                $scope.graph = toRickshaw(data.Results);
            }
            $scope.running = '';
        }).error(function (error) {
            $scope.error = error;
            $scope.running = '';
        });
        $scope.set = function () {
            $location.hash(btoa($scope.expr));
            $route.reload();
        };
        function toRickshaw(res) {
            var graph = [];
            angular.forEach(res, function (d, idx) {
                var data = [];
                angular.forEach(d.Value, function (val, ts) {
                    data.push({
                        x: +ts,
                        y: val
                    });
                });
                if (data.length == 0) {
                    return;
                }
                var name = '{';
                angular.forEach(d.Group, function (tagv, tagk) {
                    if (name.length > 1) {
                        name += ',';
                    }
                    name += tagk + '=' + tagv;
                });
                name += '}';
                var series = {
                    data: data,
                    name: name
                };
                graph[idx] = series;
            });
            return graph;
        }
    }]);
var TagSet = (function () {
    function TagSet() {
    }
    return TagSet;
})();

var TagV = (function () {
    function TagV() {
    }
    return TagV;
})();

var RateOptions = (function () {
    function RateOptions() {
    }
    return RateOptions;
})();

var Query = (function () {
    function Query(q) {
        this.aggregator = q && q.aggregator || 'sum';
        this.metric = q && q.metric || '';
        this.rate = q && q.rate || false;
        this.rateOptions = q && q.rateOptions || new RateOptions;
        if (q && !q.derivative) {
            // back compute derivative from q
            if (!this.rate) {
                this.derivative = 'gauge';
            } else if (this.rateOptions.counter) {
                this.derivative = 'counter';
            } else {
                this.derivative = 'rate';
            }
        } else {
            this.derivative = q && q.derivative || 'counter';
        }
        this.ds = q && q.ds || '';
        this.dstime = q && q.dstime || '';
        this.tags = q && q.tags || new TagSet;
        this.setDs();
        this.setDerivative();
    }
    Query.prototype.setDs = function () {
        if (this.dstime && this.ds) {
            this.downsample = this.dstime + '-' + this.ds;
        } else {
            this.downsample = '';
        }
    };
    Query.prototype.setDerivative = function () {
        var max = this.rateOptions.counterMax;
        this.rateOptions = new RateOptions();
        switch (this.derivative) {
            case "rate":
                this.rate = true;
                break;
            case "counter":
                this.rate = true;
                this.rateOptions.counter = true;
                this.rateOptions.counterMax = max;
                this.rateOptions.resetValue = 1;
                break;
            case "gauge":
                this.rate = false;
                break;
        }
    };
    return Query;
})();

var Request = (function () {
    function Request() {
        this.start = '1h-ago';
        this.queries = [];
    }
    Request.prototype.prune = function () {
        var _this = this;
        for (var i = 0; i < this.queries.length; i++) {
            angular.forEach(this.queries[i], function (v, k) {
                var qi = _this.queries[i];
                switch (typeof v) {
                    case "string":
                        if (!v) {
                            delete qi[k];
                        }
                        break;
                    case "boolean":
                        if (!v) {
                            delete qi[k];
                        }
                        break;
                    case "object":
                        if (Object.keys(v).length == 0) {
                            delete qi[k];
                        }
                        break;
                }
            });
        }
    };
    return Request;
})();

var graphRefresh;

tsafControllers.controller('GraphCtrl', [
    '$scope', '$http', '$location', '$route', '$timeout', function ($scope, $http, $location, $route, $timeout) {
        $scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
        $scope.rate_options = ["gauge", "counter", "rate"];
        var search = $location.search();
        var j = search.json;
        if (search.b64) {
            j = atob(search.b64);
        }
        var request = j ? JSON.parse(j) : new Request;
        $scope.index = parseInt($location.hash()) || 0;
        $scope.tagvs = [];
        $scope.sorted_tagks = [];
        $scope.query_p = [];
        angular.forEach(request.queries, function (q, i) {
            $scope.query_p[i] = new Query(q);
        });
        $scope.start = request.start;
        $scope.end = request.end;
        $scope.autods = search.autods != 'false';
        $scope.refresh = search.refresh == 'true';
        var duration_map = {
            "s": "s",
            "m": "m",
            "h": "h",
            "d": "d",
            "w": "w",
            "n": "M",
            "y": "y"
        };
        var isRel = /^(\d+)(\w)-ago$/;
        function RelToAbs(m) {
            return moment().utc().subtract(parseFloat(m[1]), duration_map[m[2]]).format();
        }
        function AbsToRel(s) {
            //Not strict parsing of the time format. For example, just "2014" will be valid
            var t = moment.utc(s, moment.defaultFormat).fromNow();
            return t;
        }
        function SwapTime(s) {
            if (!s) {
                return moment().utc().format();
            }
            var m = isRel.exec(s);
            if (m) {
                return RelToAbs(m);
            }
            return AbsToRel(s);
        }
        $scope.SwitchTimes = function () {
            $scope.start = SwapTime($scope.start);
            $scope.end = SwapTime($scope.end);
        };
        $scope.AddTab = function () {
            $scope.index = $scope.query_p.length;
            $scope.query_p.push(new Query);
        };
        $scope.setIndex = function (i) {
            $scope.index = i;
        };
        $scope.GetTagKByMetric = function (index) {
            $scope.tagvs[index] = new TagV;
            if ($scope.query_p[index].metric) {
                $http.get('/api/tagk/' + $scope.query_p[index].metric).success(function (data) {
                    if (!angular.isArray(data)) {
                        return;
                    }
                    var tags = {};
                    for (var i = 0; i < data.length; i++) {
                        if ($scope.query_p[index].tags) {
                            tags[data[i]] = $scope.query_p[index].tags[data[i]] || '';
                        } else {
                            tags[data[i]] = '';
                        }
                        GetTagVs(data[i], index);
                    }
                    $scope.query_p[index].tags = tags;

                    // Make sure host is always the first tag.
                    $scope.sorted_tagks[index] = Object.keys(tags);
                    $scope.sorted_tagks[index].sort(function (a, b) {
                        if (a == 'host') {
                            return 1;
                        } else if (b == 'host') {
                            return -1;
                        }
                        return a.localeCompare(b);
                    }).reverse();
                }).error(function (error) {
                    $scope.error = 'Unable to fetch metrics: ' + error;
                });
            }
        };
        if ($scope.query_p.length == 0) {
            $scope.AddTab();
        }
        $http.get('/api/metric').success(function (data) {
            $scope.metrics = data;
        }).error(function (error) {
            $scope.error = 'Unable to fetch metrics: ' + error;
        });

        function GetTagVs(k, index) {
            $http.get('/api/tagv/' + k + '/' + $scope.query_p[index].metric).success(function (data) {
                data.sort();
                $scope.tagvs[index][k] = data;
            }).error(function (error) {
                $scope.error = 'Unable to fetch metrics: ' + error;
            });
        }
        function getRequest() {
            request = new Request;
            request.start = $scope.start;
            request.end = $scope.end;
            angular.forEach($scope.query_p, function (p) {
                if (!p.metric) {
                    return;
                }
                var q = new Query(p);
                var tags = q.tags;
                q.tags = new TagSet;
                angular.forEach(tags, function (v, k) {
                    if (v && k) {
                        q.tags[k] = v;
                    }
                });
                request.queries.push(q);
            });
            return request;
        }
        $scope.Query = function () {
            var r = getRequest();
            r.prune();
            $location.search('b64', btoa(JSON.stringify(r)));
            $location.search('autods', $scope.autods ? undefined : 'false');
            $location.search('refresh', $scope.refresh ? 'true' : undefined);
            $route.reload();
        };
        request = getRequest();
        if (!request.queries.length) {
            return;
        }
        var autods = $scope.autods ? autods = '&autods=' + $('#chart').width() : '';
        function get(noRunning) {
            $timeout.cancel(graphRefresh);
            if (!noRunning) {
                $scope.running = 'Running';
            }
            $http.get('/api/graph?' + 'b64=' + btoa(JSON.stringify(request)) + autods).success(function (data) {
                $scope.result = data.Series;
                if (!$scope.result) {
                    $scope.warning = 'No Results';
                } else {
                    $scope.warning = '';
                }
                $scope.queries = data.Queries;
                $scope.running = '';
                $scope.error = '';
                var u = $location.absUrl();
                u = u.substr(0, u.indexOf('?')) + '?';
                u += 'b64=' + search.b64 + autods;
                $scope.url = u;
            }).error(function (error) {
                $scope.error = error;
                $scope.running = '';
            }).finally(function () {
                if ($scope.refresh) {
                    graphRefresh = $timeout(function () {
                        get(true);
                    }, 5000);
                }
                ;
            });
        }
        ;
        get(false);
    }]);
tsafControllers.controller('HostCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.host = search.host;
        $scope.time = search.time;
        $scope.tab = search.tab || "stats";
        $scope.idata = [];
        $scope.fsdata = [];
        $scope.metrics = [];
        $scope.mlink = function (m) {
            var r = new Request();
            var q = new Query();
            q.metric = m;
            q.tags = { 'host': $scope.host };
            r.queries.push(q);
            return r;
        };
        $scope.setTab = function (t) {
            $location.search('tab', t);
            $scope.tab = t;
        };
        $http.get('/api/metric/host/' + $scope.host).success(function (data) {
            $scope.metrics = data || [];
        });
        var autods = '&autods=100';
        var cpu_r = new Request();
        cpu_r.start = $scope.time;
        cpu_r.queries = [
            new Query({
                metric: 'os.cpu',
                derivative: 'counter',
                tags: { host: $scope.host }
            })
        ];
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)) + autods).success(function (data) {
            data.Series[0].name = 'Percent Used';
            $scope.cpu = data.Series;
        });
        var mem_r = new Request();
        mem_r.start = $scope.time;
        mem_r.queries.push(new Query({
            metric: "os.mem.total",
            tags: { host: $scope.host }
        }));
        mem_r.queries.push(new Query({
            metric: "os.mem.used",
            tags: { host: $scope.host }
        }));
        $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(mem_r)) + autods).success(function (data) {
            data.Series[1].name = "Used";
            $scope.mem_total = Math.max.apply(null, data.Series[0].data.map(function (d) {
                return d.y;
            }));
            $scope.mem = [data.Series[1]];
        });
        $http.get('/api/tagv/iface/os.net.bytes?host=' + $scope.host).success(function (data) {
            $scope.interfaces = data;
            angular.forEach($scope.interfaces, function (i, idx) {
                $scope.idata[idx] = {
                    name: i
                };
                var net_bytes_r = new Request();
                net_bytes_r.start = $scope.time;
                net_bytes_r.queries = [
                    new Query({
                        metric: "os.net.bytes",
                        rate: true,
                        rateOptions: { counter: true, resetValue: 1 },
                        tags: { host: $scope.host, iface: i, direction: "*" }
                    })
                ];
                $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)) + autods).success(function (data) {
                    angular.forEach(data.Series, function (d) {
                        d.data = d.data.map(function (dp) {
                            return { x: dp.x, y: dp.y * 8 };
                        });
                        if (d.name.indexOf("direction=out") != -1) {
                            d.data = d.data.map(function (dp) {
                                return { x: dp.x, y: dp.y * -1 };
                            });
                            d.name = "out";
                        } else {
                            d.name = "in";
                        }
                    });
                    $scope.idata[idx].data = data.Series;
                });
            });
        });
        $http.get('/api/tagv/disk/os.disk.fs.space_total?host=' + $scope.host).success(function (data) {
            angular.forEach(data, function (i, idx) {
                if (i == '/dev/shm') {
                    return;
                }
                var fs_r = new Request();
                fs_r.start = $scope.time;
                fs_r.queries.push(new Query({
                    metric: "os.disk.fs.space_total",
                    tags: { host: $scope.host, disk: i }
                }));
                fs_r.queries.push(new Query({
                    metric: "os.disk.fs.space_used",
                    tags: { host: $scope.host, disk: i }
                }));
                $scope.fsdata[idx] = {
                    name: i
                };
                $http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)) + autods).success(function (data) {
                    data.Series[1].name = 'Used';
                    var total = Math.max.apply(null, data.Series[0].data.map(function (d) {
                        return d.y;
                    }));
                    var c_val = data.Series[1].data.slice(-1)[0].y;
                    var percent_used = c_val / total * 100;
                    $scope.fsdata[idx].total = total;
                    $scope.fsdata[idx].c_val = c_val;
                    $scope.fsdata[idx].percent_used = percent_used;
                    $scope.fsdata[idx].data = [data.Series[1]];
                });
            });
        });
    }]);
tsafControllers.controller('ItemsCtrl', [
    '$scope', '$http', function ($scope, $http) {
        $http.get('/api/metric').success(function (data) {
            $scope.metrics = data;
        }).error(function (error) {
            $scope.status = 'Unable to fetch metrics: ' + error;
        });
        $http.get('/api/tagv/host').success(function (data) {
            $scope.hosts = data;
        }).error(function (error) {
            $scope.status = 'Unable to fetch hosts: ' + error;
        });
    }]);
tsafControllers.controller('RuleCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var current_alert = search.alert;
        var current_template = search.template;
        var status_map = {
            "normal": 0,
            "warning": 1,
            "critical": 2
        };
        $scope.date = search.date || '';
        $scope.time = search.time || '';
        $scope.tab = search.tab || 'results';
        $http.get('/api/config/json').success(function (data) {
            $scope.alerts = data.Alerts;
            $scope.templates = data.Templates;
        });
        try  {
            current_alert = atob(current_alert);
        } catch (e) {
            current_alert = '';
        }
        if (!current_alert) {
            var alert_def = 'alert test {\n' + '    template = test\n' + '    $t = "5m"\n' + '    $q = avg(q("avg:rate{counter,,1}:os.cpu{host=*}", $t, ""))\n' + '    crit = $q > 10\n' + '}';
            $location.search('alert', btoa(alert_def));
            return;
        }
        $scope.alert = current_alert;
        try  {
            current_template = atob(current_template);
        } catch (e) {
            current_template = '';
        }
        if (!current_template) {
            var template_def = 'template test {\n' + '    body = `<h1>Name: {{.Alert.Name}}</h1>`\n' + '    subject = `{{.Last.Status}}: {{.Alert.Name}}: {{.E .Alert.Vars.q}} on {{.Group.host}}`\n' + '}';
            $location.search('template', btoa(template_def));
            return;
        }
        $scope.template = current_template;
        $scope.shiftEnter = function ($event) {
            if ($event.keyCode == 13 && $event.shiftKey) {
                $scope.set();
            }
        };
        $scope.set = function () {
            $scope.running = "Running";
            $scope.warning = [];
            $location.search('alert', btoa($scope.alert));
            $location.search('template', btoa($scope.template));
            if (typeof $scope.date == 'object') {
                $scope.date = moment($scope.date).utc().format('YYYY-MM-DD');
            }
            $location.search('date', $scope.date || null);
            $location.search('time', $scope.time || null);
            $location.search('tab', $scope.tab || 'results');
            $http.get('/api/rule?' + 'alert=' + encodeURIComponent($scope.alert) + '&template=' + encodeURIComponent($scope.template) + '&date=' + encodeURIComponent($scope.date) + '&time=' + encodeURIComponent($scope.time)).success(function (data) {
                $scope.subject = data.Subject;
                $scope.body = data.Body;
                $scope.resultTime = moment.unix(data.Time).utc().format('YYYY-MM-DD HH:mm:ss');
                $scope.results = [];
                angular.forEach(data.Result, function (v, k) {
                    $scope.results.push({
                        group: k,
                        result: v
                    });
                });
                $scope.results.sort(function (a, b) {
                    return status_map[b.result.Status] - status_map[a.result.Status];
                });
                angular.forEach(data.Warning, function (v) {
                    $scope.warning.push(v);
                });
                $scope.running = '';
                $scope.error = '';
            }).error(function (error) {
                $scope.error = error;
                $scope.running = '';
            });
        };
        $scope.set();
    }]);
tsafControllers.controller('SilenceCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        $scope.start = search.start;
        $scope.end = search.end;
        $scope.duration = search.duration;
        $scope.alert = search.alert;
        $scope.hosts = search.hosts;
        $scope.tags = search.tags;
        $scope.edit = search.edit;
        function get() {
            $http.get('/api/silence/get').success(function (data) {
                $scope.silences = data;
            }).error(function (error) {
                $scope.error = error;
            });
        }
        get();
        function getData() {
            var tags = ($scope.tags || '').split(',');
            if ($scope.hosts) {
                tags.push('host=' + $scope.hosts.split(/[ ,|]+/).join('|'));
            }
            tags = tags.filter(function (v) {
                return v != "";
            });
            var data = {
                start: $scope.start,
                end: $scope.end,
                duration: $scope.duration,
                alert: $scope.alert,
                tags: tags.join(','),
                edit: $scope.edit
            };
            return data;
        }
        var any = search.start || search.end || search.duration || search.alert || search.hosts || search.tags;
        var state = getData();
        $scope.change = function () {
            $scope.disableConfirm = true;
        };
        if (any) {
            $scope.error = null;
            $http.post('/api/silence/set', state).success(function (data) {
                if (data.length == 0) {
                    data = [{ Name: '(none)' }];
                }
                $scope.testSilences = data;
            }).error(function (error) {
                $scope.error = error;
            });
        }
        $scope.test = function () {
            $location.search('start', $scope.start || null);
            $location.search('end', $scope.end || null);
            $location.search('duration', $scope.duration || null);
            $location.search('alert', $scope.alert || null);
            $location.search('hosts', $scope.hosts || null);
            $location.search('tags', $scope.tags || null);
            $route.reload();
        };
        $scope.confirm = function () {
            $scope.error = null;
            $scope.testSilences = null;
            state.confirm = "true";
            $http.post('/api/silence/set', state).error(function (error) {
                $scope.error = error;
            }).finally(function () {
                $scope.testSilences = null;
                get();
            });
        };
        $scope.clear = function (id) {
            if (!window.confirm('Clear this silence?')) {
                return;
            }
            $scope.error = null;
            $http.post('/api/silence/clear', { id: id }).error(function (error) {
                $scope.error = error;
            }).finally(function () {
                get();
            });
        };
        $scope.time = function (v) {
            var m = moment(v).utc();
            return m.format(timeFormat);
        };
    }]);
tsafApp.directive('tsAckGroup', function () {
    return {
        scope: {
            ack: '=',
            groups: '=tsAckGroup',
            schedule: '='
        },
        templateUrl: '/partials/ackgroup.html',
        link: function (scope, elem, attrs) {
            scope.canAckSelected = scope.ack == 'Needs Acknowldgement';
            scope.panelClass = scope.$parent.panelClass;
            scope.btoa = scope.$parent.btoa;
            scope.encode = scope.$parent.encode;
            scope.shown = {};
            scope.collapse = function (i) {
                scope.shown[i] = !scope.shown[i];
            };
            scope.click = function ($event, idx) {
                scope.collapse(idx);
                if ($event.shiftKey && scope.schedule.checkIdx != undefined) {
                    var checked = scope.groups[scope.schedule.checkIdx].checked;
                    var start = Math.min(idx, scope.schedule.checkIdx);
                    var end = Math.max(idx, scope.schedule.checkIdx);
                    for (var i = start; i <= end; i++) {
                        if (i == idx) {
                            continue;
                        }
                        scope.groups[i].checked = checked;
                    }
                }
                scope.schedule.checkIdx = idx;
                scope.update();
            };
            scope.select = function (checked) {
                for (var i = 0; i < scope.groups.length; i++) {
                    scope.groups[i].checked = checked;
                }
                scope.update();
            };
            scope.update = function () {
                scope.canCloseSelected = scope.canForgetSelected = true;
                scope.anySelected = false;
                for (var i = 0; i < scope.groups.length; i++) {
                    var g = scope.groups[i];
                    if (!g.checked) {
                        continue;
                    }
                    scope.anySelected = true;
                    if (g.Active) {
                        scope.canCloseSelected = false;
                        scope.canForgetSelected = false;
                    }
                    if (g.Status != "unknown") {
                        scope.canForgetSelected = false;
                    }
                }
            };
            scope.multiaction = function (type) {
                var url = '/action?type=' + type;
                angular.forEach(scope.groups, function (group) {
                    if (!group.checked) {
                        return;
                    }
                    if (group.AlertKey) {
                        url += '&key=' + encodeURIComponent(group.AlertKey);
                    }
                    angular.forEach(group.Children, function (child) {
                        url += '&key=' + encodeURIComponent(child.AlertKey);
                    });
                });
                return url;
            };
            scope.history = function () {
                var url = '/history?';
                angular.forEach(scope.groups, function (group) {
                    if (!group.checked) {
                        return;
                    }
                    if (group.AlertKey) {
                        url += '&key=' + encodeURIComponent(group.AlertKey);
                    }
                    angular.forEach(group.Children, function (child) {
                        url += '&key=' + encodeURIComponent(child.AlertKey);
                    });
                });
                return url;
            };
        }
    };
});

tsafApp.directive('tsState', function () {
    return {
        templateUrl: '/partials/alertstate.html',
        link: function (scope, elem, attrs) {
            scope.action = function (type) {
                var key = encodeURIComponent(scope.name);
                return '/action?type=' + type + '&key=' + key;
            };
            scope.zws = function (v) {
                return v.replace(/([,{}()])/g, '$1\u200b');
            };
        }
    };
});

tsafApp.directive('tsAck', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/ack.html'
    };
});

tsafApp.directive('tsClose', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/close.html'
    };
});

tsafApp.directive('tsForget', function () {
    return {
        restrict: 'E',
        templateUrl: '/partials/forget.html'
    };
});
tsafControllers.controller('HistoryCtrl', [
    '$scope', '$http', '$location', '$route', function ($scope, $http, $location, $route) {
        var search = $location.search();
        var keys = {};
        if (angular.isArray(search.key)) {
            angular.forEach(search.key, function (v) {
                keys[v] = true;
            });
        } else {
            keys[search.key] = true;
        }
        var status;
        $scope.shown = {};
        $scope.collapse = function (i) {
            $scope.shown[i] = !$scope.shown[i];
        };
        var selected_alerts = [];
        function done() {
            var status = $scope.schedule.Status;
            angular.forEach(status, function (v, ak) {
                if (!keys[ak]) {
                    return;
                }
                angular.forEach(v.History, function (h, i) {
                    if (i + 1 < v.History.length) {
                        h.EndTime = v.History[i + 1].Time;
                    } else {
                        h.EndTime = moment.utc();
                    }
                });
                v.History.reverse();
                var dict = {};
                dict['Name'] = ak;
                dict['History'] = v.History;
                selected_alerts.push(dict);
            });
            if (selected_alerts.length > 0) {
                $scope.alert_history = selected_alerts;
            } else {
                $scope.error = 'No Matching Alerts Found';
            }
        }
        if ($scope.schedule) {
            done();
        } else {
            $scope.refresh(done);
        }
    }]);
